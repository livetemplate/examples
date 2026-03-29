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

func TestObservabilityE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Get free ports for server and Chrome debugging
	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	// Start server
	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Start Docker Chrome container
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)
	_ = chromeCmd

	// Connect to Docker Chrome via remote debugging
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	t.Run("Initial Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		if !strings.Contains(initialHTML, "Counter: 0") {
			t.Error("Initial counter value not found")
		}

		t.Log("Initial page load verified")
	})

	t.Run("Increment Button Click", func(t *testing.T) {
		// Click the increment button using JS .click()
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('button[name="increment"]').click()`, nil),
			e2etest.WaitFor(`document.querySelector('body').innerText.includes('Counter: 1')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to click increment button: %v", err)
		}

		var html string
		err = chromedp.Run(ctx,
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to get page HTML: %v", err)
		}

		if !strings.Contains(html, "Counter: 1") {
			t.Errorf("Expected 'Counter: 1' after increment, got HTML: %s", html)
		}

		t.Log("Increment button click verified")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("All observability E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}
