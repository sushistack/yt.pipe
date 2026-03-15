//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboard_ListProjects(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	// Seed 3 projects with different SCP IDs
	seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")
	seedProjectAtStage(t, baseURL, st, "SCP-049", "pending")
	seedProjectAtStage(t, baseURL, st, "SCP-096", "scenario")

	_, err := page.Goto(baseURL + "/dashboard/")
	require.NoError(t, err)

	// Verify all 3 SCP groups appear as accordion items
	err = page.Locator("text=SCP-173").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "SCP-173 group should appear")

	err = page.Locator("text=SCP-049").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "SCP-049 group should appear")

	err = page.Locator("text=SCP-096").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "SCP-096 group should appear")
}

func TestDashboard_FilterByStage(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	// Seed projects at different stages
	seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")
	seedProject(t, baseURL, "SCP-049") // pending

	_, err := page.Goto(baseURL + "/dashboard/")
	require.NoError(t, err)

	// Select "scenario" from stage filter dropdown
	_, err = page.Locator("#stage-filter").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice("scenario"),
	})
	require.NoError(t, err)

	// Wait for HTMX update
	err = page.Locator("text=SCP-173").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "SCP-173 (scenario) should be visible")

	// SCP-049 (pending) should not be visible after filtering
	count, err := page.Locator("#project-list >> text=SCP-049").Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count, "SCP-049 (pending) should be filtered out")
}

func TestDashboard_SCPSearch(t *testing.T) {
	baseURL, st := StartTestServer(t)
	page := newPage(t)

	seedProjectAtStage(t, baseURL, st, "SCP-173", "scenario")
	seedProject(t, baseURL, "SCP-049")

	_, err := page.Goto(baseURL + "/dashboard/")
	require.NoError(t, err)

	// Type "173" in SCP search input (has hx-trigger="keyup changed delay:300ms")
	// Use PressSequentially to trigger keyup events (Fill doesn't trigger keyup)
	err = page.Locator("#scp-search").Click()
	require.NoError(t, err)
	err = page.Locator("#scp-search").PressSequentially("173")
	require.NoError(t, err)

	// Wait for HTMX debounce (300ms) + request + response
	page.WaitForTimeout(1500)

	err = page.Locator("#project-list >> text=SCP-173").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "SCP-173 should be visible after search")

	// SCP-049 should be filtered out
	visible, err := page.Locator("#project-list >> text=SCP-049").IsVisible()
	require.NoError(t, err)
	assert.False(t, visible, "SCP-049 should be filtered out by SCP search")
}

func TestDashboard_NavigateToDetail(t *testing.T) {
	baseURL, _ := StartTestServer(t)
	page := newPage(t)

	projectID := seedProject(t, baseURL, "SCP-173")

	_, err := page.Goto(baseURL + "/dashboard/")
	require.NoError(t, err)

	// Click on the project link inside the SCP accordion group
	link := page.Locator("a[href='/dashboard/projects/" + projectID + "']")
	err = link.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	require.NoError(t, err)
	err = link.Click()
	require.NoError(t, err)

	// Verify navigation to detail page
	err = page.Locator("text=SCP-173").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "should navigate to project detail page")

	// Verify we're on the detail page by checking for the back link
	err = page.Locator("text=Back to projects").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	})
	assert.NoError(t, err, "back link should be visible on detail page")
}

func TestDashboard_DeleteProject(t *testing.T) {
	baseURL, _ := StartTestServer(t)
	page := newPage(t)
	acceptDialogs(page)

	projectID := seedProject(t, baseURL, "SCP-173")

	_, err := page.Goto(baseURL + "/dashboard/projects/" + projectID)
	require.NoError(t, err)

	// Click delete button (has hx-confirm)
	err = page.Locator("text=Delete").Click()
	require.NoError(t, err)

	// After successful delete, JS redirects to /dashboard/
	err = page.Locator("h1:has-text('Projects')").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(10000),
	})
	assert.NoError(t, err, "should redirect to dashboard after delete")
}
