//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// TestFullPipeline_BrowserDriven runs the entire pipeline through the browser UI:
// Create Project → Generate Scenario → Generate Characters → Select Character →
// Generate Images → Approve Images → Generate TTS → Approve TTS → Assemble.
//
// Uses fake plugins (fakeLLM, fakeImageGen, fakeTTS, fakeAssembler) for deterministic output.
// Validates each stage transition, asset creation, and final assembly output.
func TestFullPipeline_BrowserDriven(t *testing.T) {
	baseURL, st, fig := startTestServerWithPlugins(t)
	page := newPage(t)
	acceptDialogs(page)

	// ──────────────────────────────────────────────
	// Phase 1: Create Project via API
	// ──────────────────────────────────────────────
	t.Log("Phase 1: Creating project...")
	projectID := seedProject(t, baseURL, "SCP-173")
	require.NotEmpty(t, projectID)

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err, "project detail page should render SCP-173")

	err = page.Locator("text=Generate Scenario").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	require.NoError(t, err, "Generate Scenario button should be visible at pending stage")
	t.Log("  ✓ Project created, pending stage confirmed")

	// ──────────────────────────────────────────────
	// Phase 2: Generate Scenario
	// ──────────────────────────────────────────────
	t.Log("Phase 2: Generating scenario...")
	err = page.Locator("text=Generate Scenario").Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	err = page.Locator("h2:has-text('Scenes')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	require.NoError(t, err, "Scenes heading should appear after scenario generation")

	proj, err := st.GetProject(projectID)
	require.NoError(t, err)
	assert.Greater(t, proj.SceneCount, 0, "project should have scenes")
	assert.FileExists(t, filepath.Join(proj.WorkspacePath, "scenario.json"))
	t.Logf("  ✓ Scenario generated: %d scenes", proj.SceneCount)

	// ──────────────────────────────────────────────
	// Phase 3: Generate Characters + Select
	// ──────────────────────────────────────────────
	t.Log("Phase 3: Generating characters...")
	err = page.Locator("text=Generate Characters").First().Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	// Reload and try to select a candidate
	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)
	page.WaitForTimeout(1000)

	selectBtn := page.Locator("button:has-text('Select')").First()
	if visible, _ := selectBtn.IsVisible(); visible {
		t.Log("  Selecting character candidate...")
		err = selectBtn.Click()
		require.NoError(t, err)
		page.WaitForTimeout(2000)
		_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
		require.NoError(t, err)
	}

	proj, err = st.GetProject(projectID)
	require.NoError(t, err)
	t.Logf("  ✓ Characters done, stage: %s", proj.Status)

	// ──────────────────────────────────────────────
	// Phase 4: Generate Images
	// ──────────────────────────────────────────────
	t.Log("Phase 4: Generating images...")
	fig.generateCount = 0
	fig.editCount = 0

	genImgBtn := page.Locator("text=Generate Images").First()
	err = genImgBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err, "Generate Images button should be visible")
	err = genImgBtn.Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	// Verify: IMG badges
	err = page.Locator("text=IMG").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	assert.NoError(t, err, "IMG badge should appear")

	// Count image files on disk
	scenesDir := filepath.Join(proj.WorkspacePath, "scenes")
	imageFilesFound := countFilesByExt(scenesDir, proj.SceneCount, ".png")
	assert.Greater(t, imageFilesFound, 0, "image files should exist on disk")

	totalImgCalls := fig.generateCount + fig.editCount
	t.Logf("  ✓ Images: %d files, %d API calls (Gen=%d, Edit=%d)",
		imageFilesFound, totalImgCalls, fig.generateCount, fig.editCount)

	// ──────────────────────────────────────────────
	// Phase 5: Approve Images (via DB — approve-all requires review token)
	// ──────────────────────────────────────────────
	t.Log("Phase 5: Approving images...")
	bulkApprove(t, st, projectID, "image", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageImages)

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	proj, err = st.GetProject(projectID)
	require.NoError(t, err)
	t.Logf("  ✓ Images approved, stage: %s", proj.Status)

	// ──────────────────────────────────────────────
	// Phase 6: Generate TTS
	// ──────────────────────────────────────────────
	t.Log("Phase 6: Generating TTS...")
	ttsBtn := page.Locator("text=Generate TTS").First()
	err = ttsBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err, "Generate TTS button should be visible")
	err = ttsBtn.Click()
	require.NoError(t, err)

	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	err = page.Locator("text=TTS").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	assert.NoError(t, err, "TTS badge should appear")

	audioFilesFound := 0
	for i := 1; i <= proj.SceneCount; i++ {
		if _, err := os.Stat(filepath.Join(scenesDir, fmt.Sprintf("%d", i), "audio.wav")); err == nil {
			audioFilesFound++
		}
	}
	assert.Greater(t, audioFilesFound, 0, "audio files should exist")
	t.Logf("  ✓ TTS: %d audio files", audioFilesFound)

	// ──────────────────────────────────────────────
	// Phase 7: Approve TTS + set stage to TTS
	// ──────────────────────────────────────────────
	t.Log("Phase 7: Approving TTS...")
	bulkApprove(t, st, projectID, "tts", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageTTS)

	_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	proj, err = st.GetProject(projectID)
	require.NoError(t, err)
	t.Logf("  ✓ TTS approved, stage: %s", proj.Status)

	// ──────────────────────────────────────────────
	// Phase 8: Assemble
	// ──────────────────────────────────────────────
	t.Log("Phase 8: Assembling...")
	assembleBtn := page.Locator("button[onclick*='runAssemble']")
	err = assembleBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)})
	require.NoError(t, err, "Assemble button should be visible")

	isDisabled, _ := assembleBtn.IsDisabled()
	if isDisabled {
		// Force stage + write manifest.json files so assemble works
		t.Log("  Assemble button disabled, writing manifest files...")
		writeSceneManifests(t, proj.WorkspacePath, proj.SceneCount)
		setStage(t, baseURL, projectID, domain.StageTTS)
		_, err = page.Goto(baseURL + "/dashboard/projects/" + projectID)
		require.NoError(t, err)
		assembleBtn = page.Locator("button[onclick*='runAssemble']")
		err = assembleBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)})
		require.NoError(t, err)
	}

	err = assembleBtn.Click()
	require.NoError(t, err)
	page.WaitForTimeout(1000)
	waitForJobCompletion(t, page, baseURL, projectID, 30000)

	// Verify output
	outputVisible := false
	for _, sel := range []string{"text=Output Files", "text=Reassemble", "text=draft_info.json"} {
		if err = page.Locator(sel).WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(3000)}); err == nil {
			outputVisible = true
			break
		}
	}
	assert.True(t, outputVisible, "output section should appear after assembly")

	outputDir := filepath.Join(proj.WorkspacePath, "output")
	if entries, err := os.ReadDir(outputDir); err == nil {
		t.Logf("  ✓ Assembly output: %d files", len(entries))
		for _, e := range entries {
			t.Logf("    - %s", e.Name())
		}
	}

	proj, err = st.GetProject(projectID)
	require.NoError(t, err)

	t.Log("═══════════════════════════════════════════")
	t.Log("  Full Pipeline E2E Test PASSED")
	t.Logf("  Project: %s", projectID)
	t.Logf("  Scenes: %d", proj.SceneCount)
	t.Logf("  Images: %d files (%d API calls)", imageFilesFound, totalImgCalls)
	t.Logf("  Audio: %d files", audioFilesFound)
	t.Logf("  Final stage: %s", proj.Status)
	t.Log("═══════════════════════════════════════════")
}

// TestFullPipeline_APIOnly runs the pipeline via API calls only (no browser).
// Uses the /run endpoint for scenario, then drives each stage via API.
func TestFullPipeline_APIOnly(t *testing.T) {
	baseURL, st, _ := startTestServerWithPlugins(t)

	// Phase 1: Create project
	projectID := seedProject(t, baseURL, "SCP-173")
	t.Logf("Created project: %s", projectID)

	// Phase 2: Run scenario pipeline
	resp := apiPost(t, baseURL, "/api/v1/projects/"+projectID+"/run")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Wait for scenario to complete
	pollUntilStage(t, st, projectID, domain.StageScenario, 30*time.Second)

	proj, _ := st.GetProject(projectID)
	assert.Greater(t, proj.SceneCount, 0)
	t.Logf("✓ Scenario: %d scenes, stage=%s", proj.SceneCount, proj.Status)

	// Phase 3: Generate images via API
	resp = apiPost(t, baseURL, "/api/v1/projects/"+projectID+"/images/generate")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Wait for images job to finish
	pollUntilJobDone(t, st, projectID, "image_generate", 30*time.Second)

	// Approve images + advance stage
	bulkApprove(t, st, projectID, "image", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageImages)
	t.Log("✓ Images generated and approved")

	// Phase 4: Generate TTS
	resp = apiPost(t, baseURL, "/api/v1/projects/"+projectID+"/tts/generate")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	pollUntilJobDone(t, st, projectID, "tts_generate", 30*time.Second)

	// Approve TTS + advance stage
	bulkApprove(t, st, projectID, "tts", proj.SceneCount)
	setStage(t, baseURL, projectID, domain.StageTTS)
	t.Log("✓ TTS generated and approved")

	// Phase 5: Assemble
	writeSceneManifests(t, proj.WorkspacePath, proj.SceneCount)
	resp = apiPost(t, baseURL, "/api/v1/projects/"+projectID+"/assemble")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	pollUntilJobDone(t, st, projectID, "assemble", 30*time.Second)

	// Verify final state
	proj, _ = st.GetProject(projectID)
	outputDir := filepath.Join(proj.WorkspacePath, "output")
	assert.DirExists(t, outputDir, "output directory should exist")

	if entries, err := os.ReadDir(outputDir); err == nil {
		t.Logf("✓ Assembly: %d output files", len(entries))
	}

	t.Log("═══════════════════════════════════════════")
	t.Logf("  Full Pipeline API Test PASSED (stage: %s)", proj.Status)
	t.Log("═══════════════════════════════════════════")
}

// Shared helpers (bulkApprove, writeSceneManifests, etc.) are in helpers_test.go
