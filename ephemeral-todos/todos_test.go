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

func TestEphemeralTodosE2E(t *testing.T) {
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

	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)
	_ = chromeCmd

	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	t.Run("Initial_Load_Empty", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "No todos yet") {
			t.Error("Expected empty state message")
		}
	})

	t.Run("Add_Todo", func(t *testing.T) {
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 5*time.Second),
			chromedp.SetValue(`input[name="title"]`, "Buy groceries", chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('button[type="submit"]').click()`, nil),
			e2etest.WaitFor(`document.body.innerText.includes('Buy groceries')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add todo: %v", err)
		}
	})

	t.Run("Todo_Survives_Refresh_Via_DB", func(t *testing.T) {
		// Ephemeral mode: session state is NOT persisted, but SQLite retains the
		// todo. On refresh, Mount() reloads from DB.
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to reload: %v", err)
		}

		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}

		if !strings.Contains(html, "Buy groceries") {
			t.Errorf("Expected 'Buy groceries' after refresh (loaded from DB), got: %s", html)
		}
	})
}
