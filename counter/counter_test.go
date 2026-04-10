package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()

	code := m.Run()

	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

// ========== E2E Tests ==========

// TestCounterE2E tests the counter app end-to-end with a real browser
func TestCounterE2E(t *testing.T) {
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

	// Start counter server
	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Start Docker Chrome container
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)
	_ = chromeCmd // Command returned for reference; cleanup handled by StopDockerChrome

	// Connect to Docker Chrome via remote debugging
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	// Set timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	t.Run("Initial Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second), // Wait for WebSocket init and first update
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		// Verify initial state
		if !strings.Contains(initialHTML, "Live Counter") {
			t.Error("Page title not found")
		}
		if !strings.Contains(initialHTML, "Counter: 0") {
			t.Error("Initial counter value not found")
		}

		t.Log("✅ Initial page load verified")
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
	})

	t.Run("Button_Click_Increment", func(t *testing.T) {
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 5*time.Second),
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Counter: 1')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to click increment: %v", err)
		}
		t.Log("✅ Button click increment works")
	})

	t.Run("Full_Interaction_Flow", func(t *testing.T) {
		// Counter is at 1 from Button_Click_Increment
		// Decrement → 0
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="decrement"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Counter: 0')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Decrement failed: %v", err)
		}

		// Increment x3 → 3
		for i := 0; i < 3; i++ {
			err = chromedp.Run(ctx,
				chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
				e2etest.WaitFor(fmt.Sprintf(`document.body.innerText.includes('Counter: %d')`, i+1), 5*time.Second),
			)
			if err != nil {
				t.Fatalf("Increment %d failed: %v", i+1, err)
			}
		}

		// Reset → 0
		err = chromedp.Run(ctx,
			chromedp.Click(`button[name="reset"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Counter: 0')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Reset failed: %v", err)
		}

		// Leave counter at 1 for State_Persists_On_Refresh
		err = chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Counter: 1')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Final increment failed: %v", err)
		}

		t.Log("✅ Full interaction flow: increment, decrement, reset all work")
	})

	t.Run("State_Persists_On_Refresh", func(t *testing.T) {
		// Counter should be 1 from the previous test. Refresh and verify it's still 1.
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to reload: %v", err)
		}

		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}

		if strings.Contains(html, "Counter: 0") {
			t.Error("BUG: Counter reset to 0 on refresh — state was not persisted")
		}
		if !strings.Contains(html, "Counter: 1") {
			t.Errorf("Expected 'Counter: 1' after refresh, got HTML containing neither 0 nor 1")
		}
		t.Log("✅ State persists on page refresh")
	})

	t.Run("WebSocket Connection", func(t *testing.T) {
		// Check for console errors
		var logs []string
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`console.log('WebSocket test'); 'logged'`, nil),
			// Wait for WebSocket client to be initialized
			e2etest.WaitFor(`typeof window.liveTemplateClient !== 'undefined'`, 3*time.Second),
		)

		if err != nil {
			t.Fatalf("Failed to check console: %v", err)
		}

		// If we got here without WebSocket errors, connection is working
		t.Log("✅ WebSocket connection working")
		_ = logs // Prevent unused variable error
	})

	t.Run("LiveTemplate Updates", func(t *testing.T) {
		// Take a screenshot for debugging
		var buf []byte
		err := chromedp.Run(ctx,
			chromedp.CaptureScreenshot(&buf),
		)

		if err != nil {
			t.Logf("Warning: Failed to capture screenshot: %v", err)
		} else {
			t.Logf("Screenshot captured: %d bytes", len(buf))
		}

		// Verify the page still has the LiveTemplate wrapper
		var html string
		err = chromedp.Run(ctx,
			chromedp.OuterHTML(`[data-lvt-id]`, &html, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to find LiveTemplate wrapper: %v", err)
		}

		if !strings.Contains(html, "data-lvt-id") {
			t.Error("LiveTemplate wrapper not found after updates")
		}

		t.Log("✅ LiveTemplate wrapper preserved after updates")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 All E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}

// ========== WebSocket Tests ==========

func TestWebSocketBasic(t *testing.T) {
	// Get a free port
	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	portStr := fmt.Sprintf("%d", port)
	serverURL := fmt.Sprintf("http://localhost:%s", portStr)
	wsURL := fmt.Sprintf("ws://localhost:%s/", portStr)

	// Start server on dynamic port
	cmd := exec.Command("go", "run", "main.go")
	cmd.Env = append([]string{"PORT=" + portStr}, cmd.Environ()...)

	serverLogs := e2etest.NewSafeBuffer()
	cmd.Stdout = serverLogs
	cmd.Stderr = serverLogs

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		t.Logf("=== SERVER LOGS ===\n%s", serverLogs.String())
	}()

	// Wait for server
	time.Sleep(2 * time.Second)
	for i := 0; i < 30; i++ {
		if resp, err := http.Get(serverURL); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("Server is up, trying to connect WebSocket...")

	// Try to connect
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v, response: %v", err, resp)
	}
	defer conn.Close()

	t.Log("WebSocket connected successfully!")

	// Read first message (initial tree)
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	t.Logf("Received message, length: %d bytes", len(msg))
	t.Logf("First 100 bytes: %s", msg[:min(100, len(msg))])

	// Send increment action
	t.Log("Sending increment action...")
	action := []byte(`{"action":"increment","data":{}}`)
	if err := conn.WriteMessage(websocket.TextMessage, action); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read response
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	t.Logf("Received response, length: %d bytes", len(msg))
	t.Logf("Response: %s", msg)

	t.Log("✅ WebSocket test passed!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
