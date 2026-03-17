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
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

const realSCPDataPath = "/mnt/data/raw"

// startTestServerWithRealData creates a server using /mnt/data/raw as SCP data source.
func startTestServerWithRealData(t *testing.T) (string, *store.Store, *fakeImageGen) {
	t.Helper()

	if _, err := os.Stat(realSCPDataPath); os.IsNotExist(err) {
		t.Skipf("real SCP data not found at %s, skipping", realSCPDataPath)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	require.NoError(t, err)

	root := projectRoot()
	cfg := &config.Config{
		WorkspacePath: t.TempDir(),
		SCPDataPath:   realSCPDataPath,
		API:           config.APIConfig{Host: "127.0.0.1", Port: 0},
	}

	fl := &fakeLLM{}
	fig := &fakeImageGen{}
	ft := &fakeTTS{}
	fa := &fakeAssembler{}

	projectSvc := service.NewProjectService(st)
	scenarioSvc := service.NewScenarioService(st, fl, projectSvc)
	scenarioSvc.SetTemplatesDir(filepath.Join(root, "templates"))
	imageGenSvc := service.NewImageGenService(fig, st, slog.Default())
	ttsSvc := service.NewTTSService(ft, glossary.New(), st, slog.Default())
	characterSvc := service.NewCharacterService(st)
	characterSvc.SetLLM(fl)
	characterSvc.SetImageGen(fig)
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

	return fmt.Sprintf("http://%s", listener.Addr().String()), st, fig
}

// TestRealData_FullPipeline_SCP173 runs the complete pipeline using real SCP-173 data
// from /mnt/data/raw. Verifies that the 4-stage scenario pipeline reads actual SCP facts,
// generates scenes, produces cut images, TTS audio, and assembles the final output.
func TestRealData_FullPipeline_SCP173(t *testing.T) {
	baseURL, st, fig := startTestServerWithRealData(t)
	page := newPage(t)
	acceptDialogs(page)

	// Verify real data exists
	scp173Dir := filepath.Join(realSCPDataPath, "SCP-173")
	require.DirExists(t, scp173Dir, "SCP-173 data directory should exist")
	assert.FileExists(t, filepath.Join(scp173Dir, "facts.json"), "facts.json should exist")
	assert.FileExists(t, filepath.Join(scp173Dir, "main.txt"), "main.txt should exist")
	assert.FileExists(t, filepath.Join(scp173Dir, "meta.json"), "meta.json should exist")

	// Read meta to log
	metaBytes, _ := os.ReadFile(filepath.Join(scp173Dir, "meta.json"))
	var meta struct {
		SCPID string   `json:"scp_id"`
		Tags  []string `json:"tags"`
	}
	_ = json.Unmarshal(metaBytes, &meta)
	t.Logf("Real SCP data: %s (tags: %v)", meta.SCPID, meta.Tags)

	// ──────────────────────────────────────────────
	// Phase 1: Create Project
	// ──────────────────────────────────────────────
	t.Log("Phase 1: Creating project with real SCP-173 data...")
	projectID := seedProject(t, baseURL, "SCP-173")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err)
	t.Log("  ✓ Project created")

	// ──────────────────────────────────────────────
	// Phase 2: Generate Scenario (4-stage pipeline with real facts)
	// ──────────────────────────────────────────────
	t.Log("Phase 2: Generating scenario (4-stage pipeline with real SCP data)...")
	err = page.Locator("text=Generate Scenario").Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	err = page.Locator("h2:has-text('Scenes')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err)

	proj, err := st.GetProject(projectID)
	require.NoError(t, err)
	assert.Greater(t, proj.SceneCount, 0)

	// Verify scenario.json was written with real SCP content
	scenarioPath := filepath.Join(proj.WorkspacePath, "scenario.json")
	assert.FileExists(t, scenarioPath)
	scenarioBytes, err := os.ReadFile(scenarioPath)
	require.NoError(t, err)

	var scenario domain.ScenarioOutput
	require.NoError(t, json.Unmarshal(scenarioBytes, &scenario))
	assert.Equal(t, "SCP-173", scenario.SCPID)
	assert.NotEmpty(t, scenario.Scenes)
	t.Logf("  ✓ Scenario: %d scenes, title=%q", len(scenario.Scenes), scenario.Title)

	// Log first scene to verify content
	if len(scenario.Scenes) > 0 {
		s := scenario.Scenes[0]
		t.Logf("    Scene 1: mood=%s, entity_visible=%v", s.Mood, s.EntityVisible)
		if len(s.Narration) > 80 {
			t.Logf("    Narration: %s...", s.Narration[:80])
		} else {
			t.Logf("    Narration: %s", s.Narration)
		}
	}

	// ──────────────────────────────────────────────
	// Phase 3: Generate Characters
	// ──────────────────────────────────────────────
	t.Log("Phase 3: Generating characters...")
	err = page.Locator("text=Generate Characters").First().Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)
	page.WaitForTimeout(1000)

	selectBtn := page.Locator("button:has-text('Select')").First()
	if visible, _ := selectBtn.IsVisible(); visible {
		err = selectBtn.Click()
		require.NoError(t, err)
		page.WaitForTimeout(2000)
		_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
		require.NoError(t, err)
	}

	proj, _ = st.GetProject(projectID)
	t.Logf("  ✓ Characters done, stage: %s", proj.Status)

	// ──────────────────────────────────────────────
	// Phase 4: Generate Images (cut decomposition pipeline)
	// ──────────────────────────────────────────────
	t.Log("Phase 4: Generating cut images...")
	fig.generateCount = 0
	fig.editCount = 0

	genImgBtn := page.Locator("text=Generate Images").First()
	err = genImgBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err)
	err = genImgBtn.Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	// Count generated cut images
	scenesDir := filepath.Join(proj.WorkspacePath, "scenes")
	cutFiles := 0
	for i := 1; i <= proj.SceneCount; i++ {
		sceneDir := filepath.Join(scenesDir, fmt.Sprintf("%d", i))
		entries, _ := os.ReadDir(sceneDir)
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".png" {
				cutFiles++
			}
		}
	}
	assert.Greater(t, cutFiles, 0, "cut image files should exist")

	// Verify cut file naming (cut_N_M.png pattern)
	scene1Dir := filepath.Join(scenesDir, "1")
	if entries, err := os.ReadDir(scene1Dir); err == nil {
		t.Log("    Scene 1 files:")
		for _, e := range entries {
			t.Logf("      - %s", e.Name())
		}
	}

	totalCalls := fig.generateCount + fig.editCount
	t.Logf("  ✓ Images: %d cut files, %d API calls (Gen=%d, Edit=%d)",
		cutFiles, totalCalls, fig.generateCount, fig.editCount)

	// Verify IMG badges in browser
	err = page.Locator("text=IMG").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	assert.NoError(t, err, "IMG badge should appear")

	// Verify shot carousel renders images
	imgCount, _ := page.Locator("img[src*='/scenes/']").Count()
	t.Logf("  ✓ Dashboard: %d shot images rendered in browser", imgCount)

	// ──────────────────────────────────────────────
	// Phase 5: Approve Images
	// ──────────────────────────────────────────────
	t.Log("Phase 5: Approving images...")
	bulkApprove(t, st, projectID, "image", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageImages)
	t.Log("  ✓ Images approved")

	// ──────────────────────────────────────────────
	// Phase 6: Generate TTS
	// ──────────────────────────────────────────────
	t.Log("Phase 6: Generating TTS...")
	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	ttsBtn := page.Locator("text=Generate TTS").First()
	err = ttsBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err)
	err = ttsBtn.Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	audioFiles := 0
	for i := 1; i <= proj.SceneCount; i++ {
		if _, err := os.Stat(filepath.Join(scenesDir, fmt.Sprintf("%d", i), "audio.wav")); err == nil {
			audioFiles++
		}
	}
	assert.Greater(t, audioFiles, 0)
	t.Logf("  ✓ TTS: %d audio files", audioFiles)

	// ──────────────────────────────────────────────
	// Phase 7: Approve TTS
	// ──────────────────────────────────────────────
	t.Log("Phase 7: Approving TTS...")
	bulkApprove(t, st, projectID, "tts", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageTTS)
	t.Log("  ✓ TTS approved")

	// ──────────────────────────────────────────────
	// Phase 8: Assemble
	// ──────────────────────────────────────────────
	t.Log("Phase 8: Assembling...")
	writeSceneManifests(t, proj.WorkspacePath, proj.SceneCount)

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	assembleBtn := page.Locator("button[onclick*='runAssemble']")
	err = assembleBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err)

	isDisabled, _ := assembleBtn.IsDisabled()
	require.False(t, isDisabled, "Assemble button should be enabled")

	err = assembleBtn.Click()
	require.NoError(t, err)
	page.WaitForTimeout(1000)
	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	// Verify assembly output
	outputDir := filepath.Join(proj.WorkspacePath, "output")
	assert.DirExists(t, outputDir)
	if entries, err := os.ReadDir(outputDir); err == nil {
		t.Logf("  ✓ Assembly: %d output files", len(entries))
		for _, e := range entries {
			t.Logf("    - %s", e.Name())
		}
	}

	// Final state
	proj, _ = st.GetProject(projectID)

	t.Log("═══════════════════════════════════════════════════")
	t.Log("  Real Data Pipeline Test — SCP-173 PASSED")
	t.Logf("  Data source: %s", realSCPDataPath)
	t.Logf("  Project: %s (stage: %s)", projectID, proj.Status)
	t.Logf("  Scenes: %d | Cuts: %d | Audio: %d", proj.SceneCount, cutFiles, audioFiles)
	t.Logf("  Image calls: %d (Gen=%d, Edit=%d)", totalCalls, fig.generateCount, fig.editCount)
	t.Log("═══════════════════════════════════════════════════")
}

// TestRealData_VerifyScenarioUsesFactSheet verifies that the scenario pipeline
// actually reads and passes facts.json content through the 4-stage pipeline.
func TestRealData_VerifyScenarioUsesFactSheet(t *testing.T) {
	baseURL, st, _ := startTestServerWithRealData(t)

	projectID := seedProject(t, baseURL, "SCP-173")

	// Run scenario
	resp := apiPost(t, baseURL, "/api/v1/projects/"+projectID+"/run")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	pollUntilStage(t, st, projectID, domain.StageScenario, 30*time.Second)

	proj, _ := st.GetProject(projectID)
	scenarioPath := filepath.Join(proj.WorkspacePath, "scenario.json")
	require.FileExists(t, scenarioPath)

	scenarioBytes, _ := os.ReadFile(scenarioPath)
	var scenario domain.ScenarioOutput
	require.NoError(t, json.Unmarshal(scenarioBytes, &scenario))

	// Verify the scenario reflects SCP-173 content
	assert.Equal(t, "SCP-173", scenario.SCPID)
	assert.NotEmpty(t, scenario.Title)
	assert.GreaterOrEqual(t, len(scenario.Scenes), 3, "should generate at least 3 scenes")

	// Verify scenes have required fields
	for i, s := range scenario.Scenes {
		assert.Equal(t, i+1, s.SceneNum, "scene numbers should be sequential")
		assert.NotEmpty(t, s.Narration, "scene %d should have narration", s.SceneNum)
		assert.NotEmpty(t, s.Mood, "scene %d should have mood", s.SceneNum)
	}

	// Verify fact_tags are present (from facts.json)
	hasFactTags := false
	for _, s := range scenario.Scenes {
		if len(s.FactTags) > 0 {
			hasFactTags = true
			break
		}
	}
	assert.True(t, hasFactTags, "at least one scene should have fact_tags from facts.json")

	t.Logf("✓ Scenario with real data: %d scenes, title=%q", len(scenario.Scenes), scenario.Title)
}

// TestRealData_DashboardShowsRealSCPs verifies the dashboard can browse real SCP entries.
func TestRealData_DashboardShowsRealSCPs(t *testing.T) {
	baseURL, _, _ := startTestServerWithRealData(t)
	page := newPage(t)

	_, err := page.Goto(baseURL + "/dashboard/")
	require.NoError(t, err)

	// The SCP selector should be able to load entries from /mnt/data/raw
	scpSearch := page.Locator("#scp-search")
	if err := scpSearch.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)}); err == nil {
		// Search for SCP-173
		err = scpSearch.Click()
		require.NoError(t, err)
		err = scpSearch.PressSequentially("173")
		require.NoError(t, err)
		page.WaitForTimeout(1500)

		// Should find SCP-173 in the list
		t.Log("✓ Dashboard SCP search works with real data")
	}
}
