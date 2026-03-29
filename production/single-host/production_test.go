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

func TestProductionE2E(t *testing.T) {
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

		if !strings.Contains(initialHTML, "Production Demo") {
			t.Error("Page title not found")
		}

		t.Log("Initial page load verified")
	})

	t.Run("Increment Button Click", func(t *testing.T) {
		// Click the increment button using JS .click()
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('button[name="increment"]').click()`, nil),
			e2etest.WaitFor(`document.querySelector('body').innerText.includes('1')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to click increment button: %v", err)
		}

		// Verify the counter value changed
		var counterText string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('.counter-value').innerText`, &counterText),
		)
		if err != nil {
			t.Fatalf("Failed to get counter value: %v", err)
		}

		if strings.TrimSpace(counterText) != "1" {
			t.Errorf("Expected counter value '1' after increment, got '%s'", counterText)
		}

		t.Log("Increment button click verified")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("All production E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}
