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

	t.Run("UI_Standards", func(t *testing.T) {
		var violations string
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const v = [];
				['onclick','onchange','oninput','onsubmit','onkeydown','onkeyup'].forEach(h => {
					document.querySelectorAll('[' + h + ']').forEach(el => v.push('inline ' + h + ' on <' + el.tagName.toLowerCase() + '>'));
				});
				document.querySelectorAll('[style]').forEach(el => {
					if (el.tagName !== 'INS' && el.tagName !== 'DEL' && !el.closest('[data-modal]') && !el.closest('[data-lvt-toast-stack]'))
						v.push('inline style on <' + el.tagName.toLowerCase() + '>');
				});
				if (!document.querySelector('meta[name="color-scheme"]')) v.push('missing color-scheme meta');
				if (document.documentElement.lang !== 'en') v.push('missing lang=en');
				const c = document.querySelector('.container');
				if (c && c.offsetWidth > 700) v.push('container too wide: ' + c.offsetWidth + 'px');
				return v.join('; ');
			})()`, &violations),
		)
		if err != nil {
			t.Fatalf("UI standards check failed: %v", err)
		}
		if violations != "" {
			t.Errorf("UI standard violations: %s", violations)
		}
		var cssStatus int
		chromedp.Run(ctx, chromedp.Evaluate(`(() => { const x = new XMLHttpRequest(); x.open('GET', '/livetemplate.css', false); x.send(); return x.status; })()`, &cssStatus))
		if cssStatus != 200 {
			t.Logf("Warning: Shared CSS not loading: status=%d (may not be available in CI)", cssStatus)
		}
		if err := chromedp.Run(ctx, e2etest.ValidatePicoCSS()); err != nil {
			t.Errorf("Pico CSS check failed: %v", err)
		}
	})

	t.Run("Visual_Check", func(t *testing.T) {
		// Navigate in case this subtest runs in isolation (e.g., -run Visual_Check)
		if err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Flash Messages Demo — form with input+button group, action buttons below")
	})

	t.Run("AddItemShowsSuccessFlash", func(t *testing.T) {
		// Use real form submission instead of WebSocket API bypass
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="item"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="item"]`, "Test Item", chromedp.ByQuery),
			chromedp.Click(`button[name="addItem"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Test Item')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}

		var html string
		chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))

		if !strings.Contains(html, "Test Item") {
			t.Error("Item not added")
		}
		if !strings.Contains(html, "Added item") {
			t.Error("Success flash not shown after adding item")
		}

		// Verify form input was cleared (auto-reset)
		var inputVal string
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('input[name="item"]').value`, &inputVal))
		if inputVal != "" {
			t.Errorf("Item input should be empty after submit, got %q", inputVal)
		}
	})

	t.Run("SimulateErrorShowsErrorFlash", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="simulateError"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Something went wrong')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to click or error flash not shown: %v", err)
		}
	})

	t.Run("RemoveItemWorks", func(t *testing.T) {
		// Add a fresh item to remove
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="item"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="item"]`, "Item To Remove", chromedp.ByQuery),
			chromedp.Click(`button[name="addItem"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Item To Remove')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add item for removal: %v", err)
		}

		// Count items before remove
		var beforeCount int
		chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('button[name="removeItem"]').length`, &beforeCount))

		// Click the last remove button — check table cells (not body, since flash message contains item name)
		err = chromedp.Run(ctx,
			chromedp.Click(`table tbody tr:last-child button[name="removeItem"]`, chromedp.ByQuery),
			e2etest.WaitFor(`(() => {
				const cells = document.querySelectorAll('table tbody tr td:first-child');
				return !Array.from(cells).some(td => td.textContent.includes('Item To Remove'));
			})()`, 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Remove item failed: %v", err)
		}

		// Verify flash message appeared for the removal
		var html string
		chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		if !strings.Contains(html, "Removed item") {
			t.Error("Expected 'Removed item' flash message after removal")
		}
	})

	t.Run("ClearItemsWorks", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="clearItems"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('No items')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Clear items failed: %v", err)
		}
	})
}
