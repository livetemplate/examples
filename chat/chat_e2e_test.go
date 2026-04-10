package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/lvt/testing"
)

// waitFor polls a JavaScript condition until it returns true or timeout is reached
func waitFor(condition string, timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		startTime := time.Now()
		for {
			var result bool
			err := chromedp.Evaluate(condition, &result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate condition '%s': %w", condition, err)
			}
			if result {
				return nil
			}
			if time.Since(startTime) > timeout {
				return fmt.Errorf("timeout waiting for condition '%s' after %v", condition, timeout)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// TestChatE2E tests the chat app end-to-end with a real browser
func TestChatE2E(t *testing.T) {
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

	// Start chat server using e2etest helper
	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	t.Logf("✅ Test server ready at %s", serverURL)

	// Start Docker Chrome container
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)
	_ = chromeCmd // Command returned for reference; cleanup handled by StopDockerChrome

	// Connect to Docker Chrome via remote debugging
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancelBrowser()

	// Set timeout for the entire test
	browserCtx, cancelTimeout := context.WithTimeout(browserCtx, 120*time.Second)
	defer cancelTimeout()

	// URL for Docker Chrome to access the server
	chromeTestURL := e2etest.GetChromeTestURL(serverPort)

	t.Run("Initial_Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(browserCtx,
			chromedp.Navigate(chromeTestURL),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			waitFor(`typeof window.liveTemplateClient !== 'undefined'`, 5*time.Second),
			chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		// Verify welcome message visible
		if !strings.Contains(initialHTML, "Welcome") {
			t.Errorf("Initial page should show welcome message")
		}

		// Verify join form visible
		if !strings.Contains(initialHTML, `name="username"`) {
			t.Errorf("Initial page should show username input")
		}

		// Verify no template expressions leaked
		if strings.Contains(initialHTML, "{{") {
			t.Errorf("Initial HTML contains unprocessed template expressions")
		}

		t.Logf("✅ Initial page loaded correctly")
	})

	t.Run("UI_Standards", func(t *testing.T) {
		var violations string
		err := chromedp.Run(browserCtx,
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
		chromedp.Run(browserCtx, chromedp.Evaluate(`(() => { const x = new XMLHttpRequest(); x.open('GET', '/livetemplate.css', false); x.send(); return x.status; })()`, &cssStatus))
		if cssStatus != 200 {
			t.Logf("Warning: Shared CSS not loading: status=%d (may not be available in CI)", cssStatus)
		}
	})

	t.Run("Join_Flow", func(t *testing.T) {
		var initialStatsText string
		var initialFormVisible bool
		var afterStatsText string
		var afterChatVisible bool
		var afterFormVisible bool

		err := chromedp.Run(browserCtx,
			// Capture initial state
			chromedp.Text("hgroup p", &initialStatsText, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('form[name="join"]') !== null`, &initialFormVisible),

			// Fill and submit join form
			chromedp.SetValue(`input[name="username"]`, "testuser", chromedp.ByQuery),
			chromedp.Click(`form[name="join"] button[type="submit"]`, chromedp.ByQuery),
			waitFor(`document.querySelector('.messages') !== null`, 5*time.Second),

			// Capture after-join state
			chromedp.Text("hgroup p", &afterStatsText, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('.messages') !== null`, &afterChatVisible),
			chromedp.Evaluate(`document.querySelector('form[name="join"]') !== null`, &afterFormVisible),
		)

		if err != nil {
			t.Fatalf("Join flow failed: %v", err)
		}

		// Verify initial state
		if !strings.Contains(initialStatsText, "Welcome") {
			t.Errorf("Initial stats should show welcome message, got: %q", initialStatsText)
		}
		if !initialFormVisible {
			t.Error("Join form should be visible initially")
		}

		// Verify after-join state
		if !strings.Contains(afterStatsText, "Logged in as testuser") {
			t.Errorf("After join, stats should show logged in state, got: %q", afterStatsText)
		}
		if !strings.Contains(afterStatsText, "user") && !strings.Contains(afterStatsText, "online") {
			t.Errorf("After join, stats should show online users, got: %q", afterStatsText)
		}
		if !strings.Contains(afterStatsText, "message") {
			t.Errorf("After join, stats should show message count, got: %q", afterStatsText)
		}
		if !afterChatVisible {
			t.Error("Chat interface should be visible after join")
		}
		if afterFormVisible {
			t.Error("Join form should NOT be visible after join")
		}

		t.Logf("✅ Chat join flow test passed")
		t.Logf("   Initial: %q", initialStatsText)
		t.Logf("   After:   %q", afterStatsText)
	})

	t.Run("Send_Message", func(t *testing.T) {
		var beforeHTML string
		var after1HTML string
		var after2HTML string
		var after3HTML string
		var msg1Count, msg2Count, msg3Count int
		var msg1Text, msg2Text, msg3Text string

		// Note: This test depends on Join_Flow having run first in the same browser context
		// When run standalone, we need to ensure we're in the joined state
		var isJoined bool
		chromedp.Run(browserCtx,
			chromedp.Evaluate(`document.querySelector('.messages') !== null`, &isJoined),
		)

		if !isJoined {
			t.Log("Not yet joined, performing join...")
			chromedp.Run(browserCtx,
				chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
				chromedp.SetValue(`input[name="username"]`, "testuser", chromedp.ByQuery),
				chromedp.Click(`form[name="join"] button[type="submit"]`, chromedp.ByQuery),
				waitFor(`document.querySelector('.messages') !== null`, 5*time.Second),
				chromedp.WaitVisible(`.messages`, chromedp.ByQuery),
			)
			t.Log("Join completed, .messages container is visible")
		}

		err := chromedp.Run(browserCtx,

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 1: Capturing initial state")
				return nil
			}),
			chromedp.OuterHTML(`.messages`, &beforeHTML, chromedp.ByQuery),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 2: Sending FIRST message")
				return nil
			}),
			chromedp.SetValue(`input[name="message"]`, "First message", chromedp.ByQuery),
			chromedp.Click(`form[name="send"] button[type="submit"]`, chromedp.ByQuery),
			waitFor(`document.querySelectorAll('.messages .message').length >= 1`, 5*time.Second),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 3: Checking first message")
				return nil
			}),
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &msg1Count),
			chromedp.OuterHTML(`.messages`, &after1HTML, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.message p')).map(el => el.textContent).join('|')`, &msg1Text),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Logf("After 1st: count=%d, text=%q", msg1Count, msg1Text)
				return nil
			}),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 4: Sending SECOND message")
				return nil
			}),
			chromedp.SetValue(`input[name="message"]`, "Second message", chromedp.ByQuery),
			chromedp.Click(`form[name="send"] button[type="submit"]`, chromedp.ByQuery),
			waitFor(`document.querySelectorAll('.messages .message').length >= 2`, 5*time.Second),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 5: Checking second message")
				return nil
			}),
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &msg2Count),
			chromedp.OuterHTML(`.messages`, &after2HTML, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.message p')).map(el => el.textContent).join('|')`, &msg2Text),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Logf("After 2nd: count=%d, text=%q", msg2Count, msg2Text)
				return nil
			}),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 6: Sending THIRD message")
				return nil
			}),
			chromedp.SetValue(`input[name="message"]`, "Third message", chromedp.ByQuery),
			chromedp.Click(`form[name="send"] button[type="submit"]`, chromedp.ByQuery),
			waitFor(`document.querySelectorAll('.messages .message').length >= 3`, 5*time.Second),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 7: Checking third message")
				return nil
			}),
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &msg3Count),
			chromedp.OuterHTML(`.messages`, &after3HTML, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.message p')).map(el => el.textContent).join('|')`, &msg3Text),
		)

		if err != nil {
			t.Fatalf("Send message failed: %v", err)
		}

		// Log state at each step
		t.Logf("Before: empty=%v", strings.Contains(beforeHTML, "No messages yet"))
		t.Logf("After 1st: count=%d, texts=%q", msg1Count, msg1Text)
		t.Logf("After 2nd: count=%d, texts=%q", msg2Count, msg2Text)
		t.Logf("After 3rd: count=%d, texts=%q", msg3Count, msg3Text)

		// Verify first message
		if msg1Count != 1 {
			t.Errorf("After 1st message: expected 1 message, got %d", msg1Count)
			t.Logf("HTML after 1st:\n%s", after1HTML)
		}
		if !strings.Contains(msg1Text, "First message") {
			t.Errorf("After 1st message: expected 'First message', got %q", msg1Text)
		}

		// Verify second message
		if msg2Count != 2 {
			t.Errorf("After 2nd message: expected 2 messages, got %d", msg2Count)
			t.Logf("HTML after 2nd:\n%s", after2HTML)
		}
		if !strings.Contains(msg2Text, "First message") {
			t.Errorf("After 2nd message: 'First message' missing from %q", msg2Text)
		}
		if !strings.Contains(msg2Text, "Second message") {
			t.Errorf("After 2nd message: 'Second message' missing from %q", msg2Text)
		}

		// Verify third message
		if msg3Count != 3 {
			t.Errorf("After 3rd message: expected 3 messages, got %d", msg3Count)
			t.Logf("HTML after 3rd:\n%s", after3HTML)
		}
		if !strings.Contains(msg3Text, "First message") {
			t.Errorf("After 3rd message: 'First message' missing from %q", msg3Text)
		}
		if !strings.Contains(msg3Text, "Second message") {
			t.Errorf("After 3rd message: 'Second message' missing from %q", msg3Text)
		}
		if !strings.Contains(msg3Text, "Third message") {
			t.Errorf("After 3rd message: 'Third message' missing from %q", msg3Text)
		}

		// Verify message input was cleared after submit (form auto-reset)
		var inputVal string
		chromedp.Run(browserCtx,
			chromedp.Evaluate(`document.querySelector('input[name="message"]').value`, &inputVal),
		)
		if inputVal != "" {
			t.Errorf("Message input should be empty after send, got %q", inputVal)
		}

		// Verify stats contain message count
		var statsText string
		chromedp.Run(browserCtx,
			chromedp.TextContent(`hgroup p`, &statsText, chromedp.ByQuery),
		)
		if !strings.Contains(statsText, "3") {
			t.Errorf("Stats should contain message count '3', got %q", statsText)
		}

		t.Logf("✅ Multiple message send test passed")
	})

	t.Run("WebSocket_Updates", func(t *testing.T) {
		var finalHTML string

		err := chromedp.Run(browserCtx,
			chromedp.OuterHTML(`[data-lvt-id]`, &finalHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to get final HTML: %v", err)
		}

		// Verify no template expressions leaked through
		if strings.Contains(finalHTML, "{{") {
			t.Errorf("Final HTML contains template expressions")
		}

		// Verify messages are present (from Send_Message test)
		if !strings.Contains(finalHTML, "First message") {
			t.Errorf("Final HTML should contain 'First message'")
		}
		if !strings.Contains(finalHTML, "Third message") {
			t.Errorf("Final HTML should contain 'Third message'")
		}

		t.Logf("✅ WebSocket updates working correctly")
	})

	t.Logf("\n============================================================")
	t.Logf("🎉 All Chat E2E tests passed!")
	t.Logf("============================================================")
}
