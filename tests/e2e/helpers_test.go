//go:build e2e || liveapi

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

// projectRoot returns the root directory of the yt.pipe project by walking up
// from the current file until go.mod is found. Works in both local and CI environments.
func projectRoot() string {
	// Start from working directory
	dir, err := os.Getwd()
	if err != nil {
		panic("cannot get working directory: " + err.Error())
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("cannot find project root (go.mod)")
		}
		dir = parent
	}
}

// --- Fake plugins ---

// fakeLLM implements llm.LLM with canned responses.
// Complete() detects which 4-stage pipeline stage is calling based on prompt content.
type fakeLLM struct {
	completeCallCount int
}

func (f *fakeLLM) Complete(_ context.Context, msgs []llm.Message, _ llm.CompletionOptions) (*llm.CompletionResult, error) {
	f.completeCallCount++
	prompt := ""
	if len(msgs) > 0 {
		prompt = msgs[len(msgs)-1].Content
	}

	var content string
	switch {
	// Quality Gate: Critic Agent — detect "Content Director" or "verdict" in prompt
	case strings.Contains(prompt, "Content Director") || strings.Contains(prompt, "scenario_json"):
		content = `{
			"verdict": "pass",
			"hook_effective": true,
			"retention_risk": "low",
			"ending_impact": "strong",
			"feedback": "좋은 시나리오입니다. 시청자 몰입도가 높습니다.",
			"scene_notes": []
		}`

	// 4-stage pipeline Stage 3: Writing — must return parseable JSON scenario
	// Returns 7 scenes with proper hooks and mood variation to pass Layer 1 quality gate
	case strings.Contains(prompt, "scene_structure") || strings.Contains(prompt, "03_writing") ||
		(f.completeCallCount == 3 && strings.Contains(prompt, "SCP")):
		content = `{
			"scp_id": "SCP-173",
			"title": "The Sculpture That Watches",
			"scenes": [
				{"scene_num": 1, "narration": "눈을 감는 순간, 당신은 죽습니다. Site-19의 격리실에서 가장 위험한 개체가 기다리고 있습니다.", "visual_description": "A dimly lit concrete containment chamber with heavy steel doors", "mood": "tense", "fact_tags": [{"key": "object_class", "content": "Euclid"}], "entity_visible": false},
				{"scene_num": 2, "narration": "SCP-173은 콘크리트로 만들어진 조각상입니다. 당신이 이 개체를 처음 본다면 평범한 예술품으로 착각할 수도 있습니다.", "visual_description": "A concrete humanoid statue with rebar protruding from its form, green spray paint visible", "mood": "suspense", "fact_tags": [{"key": "description", "content": "concrete sculpture"}], "entity_visible": true},
				{"scene_num": 3, "narration": "하지만 시선을 떼는 순간, 이 개체는 믿을 수 없는 속도로 이동합니다.", "visual_description": "Security camera footage showing the statue in different positions between frames", "mood": "horror", "fact_tags": [{"key": "behavior", "content": "moves when unobserved"}], "entity_visible": true},
				{"scene_num": 4, "narration": "재단은 이 개체를 격리하기 위해 최소 3명의 인원이 항상 시선을 유지하도록 규정했습니다.", "visual_description": "Three security guards maintaining eye contact with the statue", "mood": "tense", "fact_tags": [{"key": "containment", "content": "3-person visual contact"}], "entity_visible": true},
				{"scene_num": 5, "narration": "당신이 눈을 깜빡이는 그 찰나의 순간에도 SCP-173은 움직일 수 있습니다.", "visual_description": "Close-up of blinking eye with statue visible in peripheral vision", "mood": "suspense", "fact_tags": [{"key": "mechanism", "content": "moves during blink"}], "entity_visible": true},
				{"scene_num": 6, "narration": "격리실 바닥에는 정체불명의 물질이 쌓여갑니다. 재단 과학자들도 이것의 정체를 아직 밝혀내지 못했습니다.", "visual_description": "Mysterious reddish-brown substance accumulating on the chamber floor", "mood": "mystery", "fact_tags": [{"key": "anomaly", "content": "unknown substance"}], "entity_visible": false},
				{"scene_num": 7, "narration": "만약 세 명 모두가 동시에 눈을 깜빡인다면, 어떤 일이 벌어질까요? 그 답을 아는 사람은 아무도 살아남지 못했습니다.", "visual_description": "Empty containment chamber with broken neck brace visible", "mood": "horror", "fact_tags": [{"key": "lethality", "content": "neck snapping"}], "entity_visible": false}
			],
			"metadata": {"duration_estimate": "10min"}
		}`

	// 4-stage pipeline Stage 4: Review — must return parseable JSON review report
	case strings.Contains(prompt, "narration_script") || strings.Contains(prompt, "04_review") ||
		(f.completeCallCount == 4 && strings.Contains(prompt, "SCP")):
		content = `{
			"overall_pass": true,
			"coverage_pct": 85.0,
			"issues": [],
			"corrections": [],
			"storytelling_score": 82,
			"storytelling_issues": []
		}`

	// 4-stage pipeline Stages 1-2: Research/Structure — free-form text
	case strings.Contains(prompt, "research") || strings.Contains(prompt, "structure") ||
		strings.Contains(prompt, "fact_sheet"):
		content = "### Visual Identity Profile\nConcrete humanoid statue, approximately 2m tall, with rebar and green spray paint.\n\n### Key Facts\n- Object Class: Euclid\n- Animate concrete sculpture\n- Moves when unobserved\n- Hostile, capable of snapping necks"

	// Character generation (called by GenerateCandidates via Complete)
	default:
		content = `[
			{"index": 1, "name": "SCP-173", "visual_descriptor": "Concrete humanoid statue", "image_prompt": "concrete statue with rebar"},
			{"index": 2, "name": "SCP-173", "visual_descriptor": "Peanut-shaped concrete entity", "image_prompt": "peanut shaped concrete creature"},
			{"index": 3, "name": "SCP-173", "visual_descriptor": "Rebar-armed construct", "image_prompt": "industrial concrete figure with rebar arms"}
		]`
	}

	return &llm.CompletionResult{
		Content:      content,
		InputTokens:  100,
		OutputTokens: 200,
		Model:        "fake-model",
	}, nil
}

func (f *fakeLLM) GenerateScenario(_ context.Context, scpID string, _ string, _ []domain.FactTag, _ map[string]string) (*domain.ScenarioOutput, error) {
	return &domain.ScenarioOutput{
		SCPID: scpID,
		Title: "Test Scenario: " + scpID,
		Scenes: []domain.SceneScript{
			{SceneNum: 1, Narration: "Scene 1 narration for " + scpID, VisualDescription: "A dark containment chamber", Mood: "tense", FactTags: []domain.FactTag{{Key: "object_class", Content: "Euclid"}}},
			{SceneNum: 2, Narration: "Scene 2 narration for " + scpID, VisualDescription: "Security personnel approaching", Mood: "suspense", FactTags: []domain.FactTag{{Key: "containment", Content: "Class-III"}}},
			{SceneNum: 3, Narration: "Scene 3 narration for " + scpID, VisualDescription: "The entity in full view", Mood: "horror", FactTags: []domain.FactTag{{Key: "behavior", Content: "Hostile"}}},
		},
		Metadata: map[string]any{"duration_estimate": "10min"},
	}, nil
}

func (f *fakeLLM) RegenerateSection(_ context.Context, _ *domain.ScenarioOutput, sceneNum int, _ string) (*domain.SceneScript, error) {
	return &domain.SceneScript{
		SceneNum:          sceneNum,
		Narration:         fmt.Sprintf("Regenerated narration for scene %d", sceneNum),
		VisualDescription: "Regenerated visual description",
		Mood:              "neutral",
	}, nil
}

// 1x1 transparent PNG
var fakePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02,
	0x00, 0x01, 0xe5, 0x27, 0xde, 0xfc, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// fakeImageGen implements imagegen.ImageGen with call tracking.
type fakeImageGen struct {
	generateCount int
	editCount     int
}

func (f *fakeImageGen) Generate(_ context.Context, _ string, _ imagegen.GenerateOptions) (*imagegen.ImageResult, error) {
	f.generateCount++
	return &imagegen.ImageResult{
		ImageData: fakePNG,
		Format:    "png",
		Width:     1,
		Height:    1,
	}, nil
}

func (f *fakeImageGen) Edit(_ context.Context, _ []byte, _ string, _ imagegen.EditOptions) (*imagegen.ImageResult, error) {
	f.editCount++
	return &imagegen.ImageResult{
		ImageData: fakePNG,
		Format:    "png",
		Width:     1,
		Height:    1,
	}, nil
}

// Minimal WAV header (44 bytes) for a 0-sample mono 16-bit 16kHz file.
var fakeWAV = func() []byte {
	b := make([]byte, 44)
	copy(b[0:4], "RIFF")
	b[4] = 36
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	b[16] = 16 // chunk size
	b[20] = 1  // PCM
	b[22] = 1  // mono
	b[24] = 0x80
	b[25] = 0x3e // 16000 Hz
	b[28] = 0x00
	b[29] = 0x7d // byte rate = 32000
	b[32] = 2    // block align
	b[34] = 16   // bits per sample
	copy(b[36:40], "data")
	return b
}()

// fakeTTS implements tts.TTS.
type fakeTTS struct{}

func (f *fakeTTS) Synthesize(_ context.Context, _ string, _ string, _ *tts.TTSOptions) (*tts.SynthesisResult, error) {
	return &tts.SynthesisResult{
		AudioData:   fakeWAV,
		WordTimings: []domain.WordTiming{{Word: "test", StartSec: 0.0, EndSec: 0.5}},
		DurationSec: 0.5,
	}, nil
}

func (f *fakeTTS) SynthesizeWithOverrides(ctx context.Context, text string, voice string, _ map[string]string, opts *tts.TTSOptions) (*tts.SynthesisResult, error) {
	return f.Synthesize(ctx, text, voice, opts)
}

// fakeAssembler implements output.Assembler.
type fakeAssembler struct{}

func (f *fakeAssembler) Assemble(_ context.Context, input output.AssembleInput) (*output.AssembleResult, error) {
	outPath := filepath.Join(input.OutputDir, "draft_info.json")
	if err := os.MkdirAll(input.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("fake assembler: mkdir: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(`{"draft": "fake"}`), 0o644); err != nil {
		return nil, fmt.Errorf("fake assembler: write: %w", err)
	}
	return &output.AssembleResult{
		OutputPath:    input.OutputDir,
		SceneCount:    len(input.Scenes),
		TotalDuration: 10.0,
		ImageCount:    len(input.Scenes),
		AudioCount:    len(input.Scenes),
		SubtitleCount: len(input.Scenes),
	}, nil
}

func (f *fakeAssembler) Validate(_ context.Context, _ string) error {
	return nil
}

// --- Test helpers ---

// StartTestServer creates an in-process server with fake plugins and returns the base URL, store, and fakeImageGen.
func StartTestServer(t *testing.T) (string, *store.Store) {
	url, st, _ := startTestServerWithPlugins(t)
	return url, st
}

// startTestServerWithPlugins creates an in-process server and returns fake plugins for inspection.
func startTestServerWithPlugins(t *testing.T) (string, *store.Store, *fakeImageGen) {
	t.Helper()

	// Use file-based temp DB to avoid SQLite :memory: multi-connection isolation issue.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	require.NoError(t, err)

	root := projectRoot()
	cfg := &config.Config{
		WorkspacePath: t.TempDir(),
		SCPDataPath:   filepath.Join(root, "testdata"), // Use bundled test SCP data
		API:           config.APIConfig{Host: "127.0.0.1", Port: 0},
	}

	fl := &fakeLLM{}
	fig := &fakeImageGen{}
	ft := &fakeTTS{}
	fa := &fakeAssembler{}

	projectSvc := service.NewProjectService(st)
	scenarioSvc := service.NewScenarioService(st, fl, projectSvc)
	scenarioSvc.SetTemplatesDir(filepath.Join(root, "templates")) // Enable 4-stage pipeline
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

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
	return baseURL, st, fig
}

// newPage creates a new browser page with an isolated context.
func newPage(t *testing.T) playwright.Page {
	t.Helper()
	ctx, err := browser.NewContext()
	require.NoError(t, err)
	page, err := ctx.NewPage()
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })
	return page
}

// acceptDialogs registers a handler to auto-accept all native dialogs on a page.
func acceptDialogs(page playwright.Page) {
	page.OnDialog(func(d playwright.Dialog) {
		d.Accept()
	})
}

// seedProject creates a project via the API and returns its ID.
func seedProject(t *testing.T, baseURL string, scpID string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"scp_id": scpID})
	resp, err := http.Post(baseURL+"/api/v1/projects", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&envelope))
	require.True(t, envelope.Success, "API response should be successful")
	require.NotEmpty(t, envelope.Data.ID, "project id should not be empty")
	return envelope.Data.ID
}

// seedProjectAtStage creates a project and populates it with test data up to the given stage.
// Uses correct workspace directory structure: scenes/{num}/ with image.png, audio.wav files.
func seedProjectAtStage(t *testing.T, baseURL string, st *store.Store, scpID string, stage string) string {
	t.Helper()

	projectID := seedProject(t, baseURL, scpID)

	proj, err := st.GetProject(projectID)
	require.NoError(t, err)
	wsPath := proj.WorkspacePath

	if stage == domain.StagePending {
		return projectID
	}

	// --- Scenario stage: create scene manifests + workspace files ---
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 1; i <= 3; i++ {
		// DB: scene manifest with RFC3339 timestamp
		_, err := st.DB().Exec(
			`INSERT INTO scene_manifests (project_id, scene_num, content_hash, status, updated_at) VALUES (?, ?, ?, 'ready', ?)`,
			projectID, i, fmt.Sprintf("hash_%d", i), now,
		)
		require.NoError(t, err)

		// Workspace: scenes/{num}/ directory (NOT scene_003/)
		sceneDir := filepath.Join(wsPath, "scenes", fmt.Sprintf("%d", i))
		require.NoError(t, os.MkdirAll(sceneDir, 0o755))
	}

	// Workspace: scenario.json (domain.ScenarioOutput struct — no json tags)
	scenario := &domain.ScenarioOutput{
		SCPID: scpID,
		Title: "Test Scenario: " + scpID,
		Scenes: []domain.SceneScript{
			{SceneNum: 1, Narration: scpID + " is contained in a dark chamber", VisualDescription: "A dark containment chamber with " + scpID, Mood: "tense", EntityVisible: true},
			{SceneNum: 2, Narration: "Personnel approach " + scpID + " cautiously", VisualDescription: "Security personnel approaching " + scpID, Mood: "suspense", EntityVisible: true},
			{SceneNum: 3, Narration: scpID + " stands motionless in full view", VisualDescription: "The entity " + scpID + " in full view", Mood: "horror", EntityVisible: true},
		},
		Metadata: map[string]any{"duration_estimate": "10min"},
	}
	scenarioJSON, _ := json.Marshal(scenario)
	require.NoError(t, os.WriteFile(filepath.Join(wsPath, "scenario.json"), scenarioJSON, 0o644))

	// DB: update scene count
	_, err = st.DB().Exec(`UPDATE projects SET scene_count = 3 WHERE id = ?`, projectID)
	require.NoError(t, err)

	if stage == domain.StageScenario {
		setStage(t, baseURL, projectID, domain.StageScenario)
		return projectID
	}

	// --- Character stage: create candidate files + select ---
	if stageIndex(stage) >= stageIndex(domain.StageCharacter) {
		// Character candidate files go under workspace config path
		candidateDir := filepath.Join(wsPath, scpID, "characters")
		require.NoError(t, os.MkdirAll(candidateDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(candidateDir, "candidate_1.png"), fakePNG, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(candidateDir, "candidate_1.txt"),
			[]byte("Name: SCP-173\nVisual: Concrete statue\nPrompt: concrete statue"), 0o644))

		setStage(t, baseURL, projectID, domain.StageScenario)

		body, _ := json.Marshal(map[string]int{"candidate_num": 1})
		resp, err := http.Post(baseURL+"/api/v1/projects/"+projectID+"/characters/select",
			"application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "failed to select character candidate")
	}

	// --- Images stage: create image files in correct location ---
	if stageIndex(stage) >= stageIndex(domain.StageImages) {
		for i := 1; i <= 3; i++ {
			sceneDir := filepath.Join(wsPath, "scenes", fmt.Sprintf("%d", i))
			imgPath := filepath.Join(sceneDir, "image.png")
			require.NoError(t, os.WriteFile(imgPath, fakePNG, 0o644))

			// DB: update manifest + create approval
			_, err := st.DB().Exec(
				`UPDATE scene_manifests SET image_hash = 'img_hash', updated_at = ? WHERE project_id = ? AND scene_num = ?`,
				now, projectID, i,
			)
			require.NoError(t, err)
			_, _ = st.DB().Exec(
				`INSERT OR IGNORE INTO scene_approvals (project_id, scene_num, asset_type, status, attempts) VALUES (?, ?, 'image', 'approved', 1)`,
				projectID, i,
			)

			// Write scene manifest.json (required by assembly's loadScenesFromWorkspace)
			manifest := map[string]interface{}{
				"scene_num":   i,
				"narration":   fmt.Sprintf("Test narration for scene %d", i),
				"image_path":  imgPath,
				"audio_path":  "",
				"subtitle_path": "",
			}
			mdata, _ := json.Marshal(manifest)
			require.NoError(t, os.WriteFile(filepath.Join(sceneDir, "manifest.json"), mdata, 0o644))
		}
	}

	// --- TTS stage: create audio + subtitle files ---
	if stageIndex(stage) >= stageIndex(domain.StageTTS) {
		for i := 1; i <= 3; i++ {
			sceneDir := filepath.Join(wsPath, "scenes", fmt.Sprintf("%d", i))
			audioPath := filepath.Join(sceneDir, "audio.wav")
			subtitlePath := filepath.Join(sceneDir, "subtitle.json")
			imgPath := filepath.Join(sceneDir, "image.png")

			require.NoError(t, os.WriteFile(audioPath, fakeWAV, 0o644))
			require.NoError(t, os.WriteFile(subtitlePath,
				[]byte(`[{"text":"Test","start":0,"end":0.5}]`), 0o644))

			// DB: update manifest + create approval
			_, err := st.DB().Exec(
				`UPDATE scene_manifests SET audio_hash = 'audio_hash', subtitle_hash = 'sub_hash', updated_at = ? WHERE project_id = ? AND scene_num = ?`,
				now, projectID, i,
			)
			require.NoError(t, err)
			_, _ = st.DB().Exec(
				`INSERT OR IGNORE INTO scene_approvals (project_id, scene_num, asset_type, status, attempts) VALUES (?, ?, 'tts', 'approved', 1)`,
				projectID, i,
			)

			// Update scene manifest.json with audio/subtitle paths
			manifest := map[string]interface{}{
				"scene_num":      i,
				"narration":      fmt.Sprintf("Test narration for scene %d", i),
				"image_path":     imgPath,
				"audio_path":     audioPath,
				"audio_duration": 0.5,
				"subtitle_path":  subtitlePath,
				"word_timings":   []map[string]interface{}{{"Word": "test", "StartSec": 0.0, "EndSec": 0.5}},
			}
			mdata, _ := json.Marshal(manifest)
			require.NoError(t, os.WriteFile(filepath.Join(sceneDir, "manifest.json"), mdata, 0o644))
		}
	}

	setStage(t, baseURL, projectID, stage)
	return projectID
}

// setStage sets a project's stage via the API.
func setStage(t *testing.T, baseURL, projectID, stage string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"stage": stage})
	req, err := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/projects/"+projectID+"/stage", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "failed to set stage to %s", stage)
}

// stageIndex returns the ordinal position of a stage for comparison.
func stageIndex(stage string) int {
	return domain.StageIndex(stage)
}

// waitForJobCompletion waits for an async job to complete by polling the project detail page.
// Returns when scenes appear or the page shows updated content.
func waitForJobCompletion(t *testing.T, page playwright.Page, baseURL, projectID string, timeout float64) {
	t.Helper()
	// Poll by reloading until the job completes (no more loading spinner)
	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	for time.Now().Before(deadline) {
		page.WaitForTimeout(2000)
		_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
		if err != nil {
			continue
		}
		// Check if loading spinner is gone (job completed)
		spinnerCount, _ := page.Locator(".loading-spinner").Count()
		if spinnerCount == 0 {
			return
		}
	}
}

// --- Shared pipeline test helpers ---

// bulkApprove approves all scenes for a given asset type directly in the DB.
func bulkApprove(t *testing.T, st *store.Store, projectID, assetType string, sceneCount int) {
	t.Helper()
	for i := 1; i <= sceneCount; i++ {
		_, err := st.DB().Exec(
			`INSERT OR REPLACE INTO scene_approvals (project_id, scene_num, asset_type, status, attempts) VALUES (?, ?, ?, 'approved', 1)`,
			projectID, i, assetType,
		)
		require.NoError(t, err)
	}
}

// writeSceneManifests writes manifest.json files needed by the assembler.
func writeSceneManifests(t *testing.T, wsPath string, sceneCount int) {
	t.Helper()
	for i := 1; i <= sceneCount; i++ {
		sceneDir := filepath.Join(wsPath, "scenes", fmt.Sprintf("%d", i))
		_ = os.MkdirAll(sceneDir, 0o755)

		imgPath := findFirstFile(sceneDir, []string{"*.png", "*.jpg", "*.webp"})
		audioPath := filepath.Join(sceneDir, "audio.wav")
		subtitlePath := filepath.Join(sceneDir, "subtitle.json")

		manifest := map[string]interface{}{
			"scene_num":      i,
			"narration":      fmt.Sprintf("Narration for scene %d", i),
			"image_path":     imgPath,
			"audio_path":     audioPath,
			"audio_duration": 0.5,
			"subtitle_path":  subtitlePath,
			"word_timings":   []map[string]interface{}{{"Word": "test", "StartSec": 0.0, "EndSec": 0.5}},
		}
		data, _ := json.Marshal(manifest)
		_ = os.WriteFile(filepath.Join(sceneDir, "manifest.json"), data, 0o644)
	}
}

// findFirstFile returns the first file matching any of the patterns in a directory.
func findFirstFile(dir string, patterns []string) string {
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

// countFilesByExt counts files with the given extension across scene directories.
func countFilesByExt(scenesDir string, sceneCount int, ext string) int {
	count := 0
	for i := 1; i <= sceneCount; i++ {
		sceneDir := filepath.Join(scenesDir, fmt.Sprintf("%d", i))
		entries, _ := os.ReadDir(sceneDir)
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ext {
				count++
			}
		}
	}
	return count
}

// pollUntilStage polls the project until it reaches the expected stage or times out.
func pollUntilStage(t *testing.T, st *store.Store, projectID, expectedStage string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		proj, err := st.GetProject(projectID)
		if err != nil {
			continue
		}
		if stageIndex(proj.Status) >= stageIndex(expectedStage) {
			return
		}
	}
	t.Fatalf("timeout waiting for stage %s", expectedStage)
}

// pollUntilJobDone polls until no running job of the given type exists.
func pollUntilJobDone(t *testing.T, st *store.Store, projectID, jobType string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		jobs, err := st.ListJobsByProject(projectID)
		if err != nil {
			continue
		}
		running := false
		for _, j := range jobs {
			if j.Type == jobType && (j.Status == "running" || j.Status == "pending") {
				running = true
				break
			}
		}
		if !running {
			return
		}
	}
	t.Fatalf("timeout waiting for job %s to complete", jobType)
}

func apiPost(t *testing.T, baseURL, path string) *http.Response {
	t.Helper()
	resp, err := http.Post(baseURL+path, "application/json", nil)
	require.NoError(t, err)
	return resp
}
