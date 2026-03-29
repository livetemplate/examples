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

func TestFlashMessagesE2E(t *testing.T) {
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

	t.Run("InitialLoad", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Flash Messages Demo") {
			t.Error("Page title not found")
		}
	})

	t.Run("AddItemShowsSuccessFlash", func(t *testing.T) {
		// Use WebSocket API directly to bypass any client form interception issues
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'addItem', data: {item: 'Test Item'}})`, nil),
			chromedp.Sleep(2*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to send: %v", err)
		}

		var html string
		chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		t.Logf("Has 'Test Item' in page: %v", strings.Contains(html, "Test Item"))
		t.Logf("Has 'Added item' flash: %v", strings.Contains(html, "Added item"))
		// Also check the flash div content directly
		var flashContent string
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('[data-flash="success"]')?.textContent || 'NOT FOUND'`, &flashContent))
		t.Logf("Flash success div content: %q", flashContent)
		// Dump the flash area of the DOM
		var flashArea string
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('[data-lvt-id]')?.innerHTML?.substring(0, 1000) || 'NO WRAPPER'`, &flashArea))
		t.Logf("Wrapper HTML (first 1000): %s", flashArea)

		if !strings.Contains(html, "Test Item") {
			t.Error("Item not added")
		}
		if !strings.Contains(html, "Added item") {
			t.Error("Success flash not shown after adding item")
		}
	})

	t.Run("SimulateErrorShowsErrorFlash", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('button[name="simulateError"]').click()`, nil),
			chromedp.Sleep(2*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to click: %v", err)
		}

		var html string
		chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		t.Logf("Has 'Something went wrong' flash: %v", strings.Contains(html, "Something went wrong"))

		if !strings.Contains(html, "Something went wrong") {
			t.Error("Error flash not shown after simulate error")
		}
	})

	t.Run("ClearItemsWorks", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('button[name="clearItems"]').click()`, nil),
			e2etest.WaitFor(`document.body.innerText.includes('No items')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Clear items failed: %v", err)
		}
	})
}
