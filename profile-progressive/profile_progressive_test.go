package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()
	code := m.Run()
	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

func TestProfileProgressiveE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	e2etest.StartTestServer(t, "main.go", serverPort)

	if err := e2etest.StartDockerChrome(t, debugPort); err != nil {
		t.Fatalf("Failed to start Docker Chrome: %v", err)
	}
	defer e2etest.StopDockerChrome(t, debugPort)

	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err = chromedp.Run(ctx,
		chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
	)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	t.Run("InitialLoad", func(t *testing.T) {
		var html string
		var hasSuccess bool
		var previewHTML string
		err := chromedp.Run(ctx,
			chromedp.OuterHTML("body", &html, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('.success') !== null`, &hasSuccess),
			chromedp.OuterHTML(".preview", &previewHTML, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to get initial state: %v", err)
		}

		if !strings.Contains(html, "Edit Profile") {
			t.Error("Page heading not found")
		}
		if !strings.Contains(html, "Jane Doe") {
			t.Error("Initial display name not found")
		}
		if !strings.Contains(html, "jane@example.com") {
			t.Error("Initial email not found")
		}
		if hasSuccess {
			t.Error("Success message should not be visible initially")
		}
		if !strings.Contains(previewHTML, "Jane Doe") {
			t.Error("Preview should show initial name")
		}
	})

	t.Run("ValidSave", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Clear(`input[name="DisplayName"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="DisplayName"]`, "John Smith", chromedp.ByQuery),
			chromedp.Clear(`input[name="Email"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="Email"]`, "john@example.com", chromedp.ByQuery),
			chromedp.Clear(`textarea[name="Bio"]`, chromedp.ByQuery),
			chromedp.SendKeys(`textarea[name="Bio"]`, "Updated bio text.", chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('button[type="submit"]').click()`, nil),
			e2etest.WaitFor(`document.querySelector('.success') !== null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to save profile: %v", err)
		}

		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("body", &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if !strings.Contains(html, "Profile saved successfully") {
			t.Error("Success message not found")
		}

		var previewHTML string
		if err := chromedp.Run(ctx, chromedp.OuterHTML(".preview", &previewHTML, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get preview: %v", err)
		}
		if !strings.Contains(previewHTML, "John Smith") {
			t.Error("Preview should show new name")
		}
		if !strings.Contains(previewHTML, "john@example.com") {
			t.Error("Preview should show new email")
		}
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'submit', data: {DisplayName: 'Jane Updated', Email: 'jane.updated@example.com', Bio: 'Fixed and saved.'}})`, nil),
			e2etest.WaitFor(`document.querySelector('.success') !== null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to update profile: %v", err)
		}

		var previewHTML string
		if err := chromedp.Run(ctx, chromedp.OuterHTML(".preview", &previewHTML, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get preview: %v", err)
		}
		if !strings.Contains(previewHTML, "Jane Updated") {
			t.Error("Preview should show updated name")
		}
		if !strings.Contains(previewHTML, "jane.updated@example.com") {
			t.Error("Preview should show updated email")
		}
	})

	t.Run("LiveTemplateWrapper", func(t *testing.T) {
		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML(`[data-lvt-id]`, &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to find LiveTemplate wrapper: %v", err)
		}
		if !strings.Contains(html, "data-lvt-id") {
			t.Error("LiveTemplate wrapper not preserved")
		}
	})
}
