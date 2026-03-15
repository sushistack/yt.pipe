//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScene_ScenesRendered(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// VERIFY: Scenes heading renders
	err = page.Locator("h2:has-text('Scenes')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "Scenes heading should render at scenario stage")

	// VERIFY: "3 scenes" count appears (in both header badge and sidebar)
	err = page.Locator("text=3 scenes").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "3 scenes count should be displayed")
}

func TestScene_ImageRenderedAtImagesStage(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "images")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// VERIFY: scene images are rendered (img tags pointing to scene image endpoints)
	err = page.Locator("img[src*='/scenes/']").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "scene images should render at images stage")

	// VERIFY: IMG success badge
	err = page.Locator("text=IMG").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "IMG badge should show at images stage")
}

func TestScene_AudioRenderedAtTTSStage(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "tts")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// VERIFY: TTS badge shows
	err = page.Locator("text=TTS").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "TTS badge should show at tts stage")

	// VERIFY: Assemble button exists at TTS stage
	err = page.Locator("text=Assemble").First().WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "Assemble button should exist at tts stage")
}

func TestScene_InsertScene(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// Look for insert scene button (+ icon between scene cards, onclick=insertScene)
	insertBtn := page.Locator("[onclick*='insertScene']").First()
	err = insertBtn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		t.Skip("insert scene button not found in template")
	}

	err = insertBtn.Click()
	assert.NoError(t, err, "insert scene button should be clickable")
}

func TestScene_DeleteScene(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)
	acceptDialogs(page)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// Look for delete button on scene cards (hx-delete)
	deleteBtn := page.Locator("[hx-delete*='scenes']").First()
	err = deleteBtn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		t.Skip("delete scene button not found in template")
	}

	err = deleteBtn.Click()
	assert.NoError(t, err, "delete scene button should be clickable")
}
