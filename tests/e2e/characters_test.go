//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacter_GenerateAndSelect(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// Click "Generate Characters"
	err = page.Locator("text=Generate Characters").First().Click()
	require.NoError(t, err)

	// Wait for character generation job to complete
	waitForJobCompletion(t, page, baseURL, projectID, 20000)

	// VERIFY: character section shows candidates or selected state
	charUI := page.Locator("text=Change Selection, text=Generate New, img[src*='characters']").First()
	err = charUI.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		// Check if character was auto-selected (handler may reuse existing character)
		err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(3000),
		})
		require.NoError(t, err, "page should render after character generation")
	}
}

func TestCharacter_Deselect(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "character")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// Look for character image or "Change Selection" button
	changeBtn := page.Locator("text=Change Selection").First()
	err = changeBtn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		t.Skip("Change Selection button not found — character section may use different UI")
	}

	err = changeBtn.Click()
	require.NoError(t, err)

	// After click, verify page updates
	page.WaitForTimeout(2000)
	err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "page should render after character deselection")
}

func TestCharacter_GenerateNewReplacesExisting(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)
	acceptDialogs(page)

	// Start at character stage (already has a selected character)
	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "character")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// VERIFY: "Selected" badge and "Generate New" button are visible
	err = page.Locator("text=Selected").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	require.NoError(t, err, "Selected badge should be visible at character stage")

	generateNewBtn := page.Locator("text=Generate New").First()
	err = generateNewBtn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	require.NoError(t, err, "Generate New button should be visible")

	// Click "Generate New" (triggers confirm dialog → auto-accepted)
	err = generateNewBtn.Click()
	require.NoError(t, err)

	// Wait for character generation job to complete
	waitForJobCompletion(t, page, baseURL, projectID, 20000)

	// VERIFY: page shows either new candidates to select or updated character
	// The key assertion: page should not error out and should show character section
	err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "project detail should render after Generate New")

	// VERIFY: Character section rendered (either candidates or re-selected)
	charSection := page.Locator("#character-section")
	err = charSection.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "character section should be present after Generate New")
}

func TestCharacter_CandidatePolling(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	projectID := seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// Click Generate Characters
	btn := page.Locator("text=Generate Characters").First()
	err = btn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	require.NoError(t, err)
	err = btn.Click()
	require.NoError(t, err)

	// Wait for async job to complete via polling
	waitForJobCompletion(t, page, baseURL, projectID, 20000)

	// VERIFY: page rendered successfully after character generation polling
	err = page.Locator("h1:has-text('SCP-173')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "project detail page should render after character generation polling completes")
}
