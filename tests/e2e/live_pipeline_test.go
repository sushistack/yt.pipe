//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

// requireEnv returns the env value or skips the test.
func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("$%s not set — skipping live API test", key)
	}
	return v
}

// startLiveServer creates a server with real LLM/ImageGen/TTS providers.
// Skips the test if any API key is missing.
func startLiveServer(t *testing.T) (string, *store.Store) {
	t.Helper()

	llmKey := requireEnv(t, "YTP_LLM_API_KEY")
	imgKey := requireEnv(t, "YTP_IMAGEGEN_API_KEY")
	ttsKey := requireEnv(t, "YTP_TTS_API_KEY")

	if _, err := os.Stat(realSCPDataPath); os.IsNotExist(err) {
		t.Skipf("SCP data not found at %s", realSCPDataPath)
	}

	dbPath := filepath.Join(t.TempDir(), "live.db")
	st, err := store.New(dbPath)
	require.NoError(t, err)

	root := projectRoot()
	cfg := &config.Config{
		WorkspacePath: t.TempDir(),
		SCPDataPath:   realSCPDataPath,
		API:           config.APIConfig{Host: "127.0.0.1", Port: 0},
	}

	// Real LLM (Gemini)
	llmRaw, err := llm.GeminiFactory(map[string]interface{}{
		"api_key":    llmKey,
		"model":      "gemini-2.0-flash",
		"max_tokens": 16384,
	})
	require.NoError(t, err)
	llmProvider := llmRaw.(llm.LLM)

	// Real ImageGen (SiliconFlow FLUX)
	imgRaw, err := imagegen.SiliconFlowFactory(map[string]interface{}{
		"api_key": imgKey,
		"model":   "black-forest-labs/FLUX.1-schnell",
	})
	require.NoError(t, err)
	imgProvider := imgRaw.(imagegen.ImageGen)

	// Real TTS (DashScope)
	ttsRaw, err := tts.DashScopeFactory(map[string]interface{}{
		"api_key": ttsKey,
		"model":   "qwen3-tts-flash",
	})
	require.NoError(t, err)
	ttsProvider := ttsRaw.(tts.TTS)

	// Assembler: use fake (CapCut assembly is local-only, no API)
	fa := &fakeAssembler{}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	projectSvc := service.NewProjectService(st)
	scenarioSvc := service.NewScenarioService(st, llmProvider, projectSvc)
	scenarioSvc.SetTemplatesDir(filepath.Join(root, "templates"))
	imageGenSvc := service.NewImageGenService(imgProvider, st, logger)
	ttsSvc := service.NewTTSService(ttsProvider, glossary.New(), st, logger)
	characterSvc := service.NewCharacterService(st)
	characterSvc.SetLLM(llmProvider)
	characterSvc.SetImageGen(imgProvider)
	assemblerSvc := service.NewAssemblerService(fa, projectSvc)

	srv := api.NewServer(st, cfg,
		api.WithScenarioService(scenarioSvc),
		api.WithImageGenService(imageGenSvc),
		api.WithTTSService(ttsSvc),
		api.WithCharacterService(characterSvc),
		api.WithAssemblerService(assemblerSvc),
		api.WithPluginStatus(map[string]bool{
			"llm": true, "imagegen": true, "tts": true, "output": true,
		}),
	)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	httpSrv := &http.Server{Handler: srv.Router()}
	go httpSrv.Serve(listener)
	t.Cleanup(func() {
		httpSrv.Close()
		st.Close()
	})

	return fmt.Sprintf("http://%s", listener.Addr().String()), st
}

// TestLive_FullPipeline_SCP173 runs the entire pipeline with REAL API providers.
//
// Requires environment variables:
//
//	YTP_LLM_API_KEY      — Gemini API key
//	YTP_IMAGEGEN_API_KEY  — SiliconFlow API key
//	YTP_TTS_API_KEY       — DashScope API key
//
// Run: go test -tags=e2e -run TestLive -timeout=600s ./tests/e2e/...
func TestLive_FullPipeline_SCP173(t *testing.T) {
	baseURL, st := startLiveServer(t)
	page := newPage(t)
	acceptDialogs(page)

	t.Log("════════════════════════════════════════════")
	t.Log("  LIVE API Pipeline Test — SCP-173")
	t.Log("  LLM: Gemini | IMG: SiliconFlow FLUX | TTS: DashScope")
	t.Log("════════════════════════════════════════════")

	// ── Phase 1: Create Project ──
	t.Log("\n▶ Phase 1: Create project")
	projectID := seedProject(t, baseURL, "SCP-173")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)
	err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err)
	t.Log("  ✓ Project created")

	// ── Phase 2: Generate Scenario (real Gemini) ──
	t.Log("\n▶ Phase 2: Generate scenario (Gemini 4-stage pipeline)")
	start := time.Now()

	err = page.Locator("text=Generate Scenario").Click()
	require.NoError(t, err)

	// Real LLM needs more time — 4 stages × Gemini API calls
	waitForJobCompletion(t, page, baseURL, projectID, 120000)

	err = page.Locator("h2:has-text('Scenes')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(15000),
	})
	require.NoError(t, err)

	proj, err := st.GetProject(projectID)
	require.NoError(t, err)
	require.Greater(t, proj.SceneCount, 0)

	// Read and validate the real scenario
	scenarioBytes, _ := os.ReadFile(filepath.Join(proj.WorkspacePath, "scenario.json"))
	var scenario domain.ScenarioOutput
	require.NoError(t, json.Unmarshal(scenarioBytes, &scenario))

	t.Logf("  ✓ Scenario: %d scenes in %v", len(scenario.Scenes), time.Since(start).Round(time.Second))
	t.Logf("    Title: %s", scenario.Title)
	for _, s := range scenario.Scenes {
		narr := s.Narration
		if len(narr) > 60 {
			narr = narr[:60] + "…"
		}
		t.Logf("    Scene %d [%s] entity=%v: %s", s.SceneNum, s.Mood, s.EntityVisible, narr)
	}

	assert.GreaterOrEqual(t, len(scenario.Scenes), 7, "real LLM should generate at least 7 scenes")

	// ── Phase 3: Generate Characters (real Gemini + SiliconFlow) ──
	t.Log("\n▶ Phase 3: Generate characters")
	start = time.Now()

	err = page.Locator("text=Generate Characters").First().Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 120000)

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)
	page.WaitForTimeout(2000)

	// Select first candidate
	selectBtn := page.Locator("button:has-text('Select')").First()
	if visible, _ := selectBtn.IsVisible(); visible {
		err = selectBtn.Click()
		require.NoError(t, err)
		page.WaitForTimeout(3000)
		_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
		require.NoError(t, err)
	}

	proj, _ = st.GetProject(projectID)
	t.Logf("  ✓ Characters in %v, stage: %s", time.Since(start).Round(time.Second), proj.Status)

	// ── Phase 4: Generate Images (real SiliconFlow FLUX) ──
	t.Log("\n▶ Phase 4: Generate images (SiliconFlow FLUX)")
	start = time.Now()

	genImgBtn := page.Locator("text=Generate Images").First()
	err = genImgBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err)
	err = genImgBtn.Click()
	require.NoError(t, err)

	// Real image gen: ~5s per image × ~15 cuts
	waitForJobCompletion(t, page, baseURL, projectID, 300000)

	// Count images
	scenesDir := filepath.Join(proj.WorkspacePath, "scenes")
	totalImages := 0
	for i := 1; i <= proj.SceneCount; i++ {
		sceneDir := filepath.Join(scenesDir, fmt.Sprintf("%d", i))
		entries, _ := os.ReadDir(sceneDir)
		imgCount := 0
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".png" || filepath.Ext(e.Name()) == ".jpg" {
				imgCount++
			}
		}
		totalImages += imgCount
		if imgCount > 0 {
			t.Logf("    Scene %d: %d cut images", i, imgCount)
		}
	}

	assert.Greater(t, totalImages, 0, "should have generated images")
	t.Logf("  ✓ Images: %d total in %v", totalImages, time.Since(start).Round(time.Second))

	// Check IMG badges
	err = page.Locator("text=IMG").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	assert.NoError(t, err)

	// Verify real image sizes (not 1px fakes)
	scene1Dir := filepath.Join(scenesDir, "1")
	if entries, _ := os.ReadDir(scene1Dir); len(entries) > 0 {
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".png" || filepath.Ext(e.Name()) == ".jpg" {
				info, _ := e.Info()
				t.Logf("    %s: %d KB", e.Name(), info.Size()/1024)
				assert.Greater(t, info.Size(), int64(10000), "real image should be >10KB")
				break
			}
		}
	}

	// ── Phase 5: Approve Images ──
	t.Log("\n▶ Phase 5: Approve images")
	bulkApprove(t, st, projectID, "image", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageImages)
	t.Log("  ✓ Images approved")

	// ── Phase 6: Generate TTS (real DashScope) ──
	t.Log("\n▶ Phase 6: Generate TTS (DashScope)")
	start = time.Now()

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	ttsBtn := page.Locator("text=Generate TTS").First()
	err = ttsBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err)
	err = ttsBtn.Click()
	require.NoError(t, err)

	// Real TTS: ~3-5s per scene
	waitForJobCompletion(t, page, baseURL, projectID, 180000)

	audioCount := 0
	var totalAudioBytes int64
	for i := 1; i <= proj.SceneCount; i++ {
		audioPath := filepath.Join(scenesDir, fmt.Sprintf("%d", i), "audio.wav")
		if info, err := os.Stat(audioPath); err == nil {
			audioCount++
			totalAudioBytes += info.Size()
		}
	}
	assert.Greater(t, audioCount, 0)
	t.Logf("  ✓ TTS: %d files, %.1f MB total in %v",
		audioCount, float64(totalAudioBytes)/1024/1024, time.Since(start).Round(time.Second))

	// ── Phase 7: Approve TTS ──
	t.Log("\n▶ Phase 7: Approve TTS")
	bulkApprove(t, st, projectID, "tts", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageTTS)
	t.Log("  ✓ TTS approved")

	// ── Phase 8: Assemble ──
	t.Log("\n▶ Phase 8: Assemble")
	writeSceneManifests(t, proj.WorkspacePath, proj.SceneCount)

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	assembleBtn := page.Locator("button[onclick*='runAssemble']")
	err = assembleBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err)

	if disabled, _ := assembleBtn.IsDisabled(); !disabled {
		err = assembleBtn.Click()
		require.NoError(t, err)
		page.WaitForTimeout(1000)
		waitForJobCompletion(t, page, baseURL, projectID, 30000)
	}

	outputDir := filepath.Join(proj.WorkspacePath, "output")
	if entries, err := os.ReadDir(outputDir); err == nil {
		t.Logf("  ✓ Assembly: %d output files", len(entries))
	}

	// ── Final summary ──
	proj, _ = st.GetProject(projectID)

	t.Log("\n════════════════════════════════════════════")
	t.Log("  LIVE Pipeline Complete!")
	t.Logf("  Project: %s", projectID)
	t.Logf("  Stage: %s", proj.Status)
	t.Logf("  Scenes: %d", proj.SceneCount)
	t.Logf("  Cut images: %d", totalImages)
	t.Logf("  Audio files: %d (%.1f MB)", audioCount, float64(totalAudioBytes)/1024/1024)
	t.Logf("  Workspace: %s", proj.WorkspacePath)
	t.Log("════════════════════════════════════════════")
}
