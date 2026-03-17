//go:build e2e || liveapi

package e2e

import (
	"os"
	"testing"

	"github.com/playwright-community/playwright-go"
)

// Package-level shared instances, initialized once in TestMain before any tests run.
// Browser is safe for concurrent use — each test creates an isolated BrowserContext via newPage().
// Do NOT use t.Parallel() across tests that share this browser without verifying thread safety.
var (
	pw      *playwright.Playwright
	browser playwright.Browser
)

func TestMain(m *testing.M) {
	var err error
	pw, err = playwright.Run()
	if err != nil {
		panic("failed to start playwright: " + err.Error())
	}

	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		pw.Stop()
		panic("failed to launch browser: " + err.Error())
	}

	code := m.Run()

	browser.Close()
	pw.Stop()
	os.Exit(code)
}
