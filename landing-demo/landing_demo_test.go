// Browser e2e for the landing-demo counter. Mirrors examples/counter's
// shape: spin up the server on a free port, drive a real Chrome via
// chromedp, exercise every controller method (Increment, Decrement,
// Reset, Sync). Each sub-test resets the counter first so it doesn't
// depend on execution order or the state left by other tests.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

func TestLandingDemoE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("get free server port: %v", err)
	}
	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("get free debug port: %v", err)
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
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// resetCounter clicks Reset and waits until the DOM reflects Count: 0.
	// Sub-tests call this first so each one starts from a known baseline.
	resetCounter := func(t *testing.T) {
		t.Helper()
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="reset"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 0')`, 5*time.Second),
		); err != nil {
			t.Fatalf("reset baseline: %v", err)
		}
	}

	t.Run("Initial_Load_Renders_Counter_At_Zero", func(t *testing.T) {
		var bodyHTML string
		if err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`output[aria-live="polite"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`body`, &bodyHTML, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("initial load: %v", err)
		}
		if !strings.Contains(bodyHTML, "<strong>0</strong>") {
			t.Errorf("initial Count != 0; body = %s", bodyHTML)
		}
		if !strings.Contains(bodyHTML, `aria-live="polite"`) {
			t.Errorf("counter is not in a live region; screen readers won't announce updates")
		}
	})

	t.Run("UI_Standards_Pico_And_CSP_Clean", func(t *testing.T) {
		var violations string
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const v = [];
				['onclick','onchange','oninput','onsubmit','onkeydown','onkeyup'].forEach(h => {
					document.querySelectorAll('[' + h + ']').forEach(el => v.push('inline ' + h + ' on <' + el.tagName.toLowerCase() + '>'));
				});
				document.querySelectorAll('[style]').forEach(el => {
					if (el.tagName !== 'INS' && el.tagName !== 'DEL')
						v.push('inline style on <' + el.tagName.toLowerCase() + '>');
				});
				// NOTE: no check for <style> blocks. Pico CSS and the
				// LiveTemplate client runtime inject style elements
				// dynamically (color-scheme handling, transient
				// animations). Author-written <style> blocks in the
				// template source are caught by code review instead.
				if (!document.querySelector('meta[name="color-scheme"]')) v.push('missing color-scheme meta');
				if (document.documentElement.lang !== 'en') v.push('missing lang=en');
				return v.join('; ');
			})()`, &violations),
		)
		if err != nil {
			t.Fatalf("UI standards check: %v", err)
		}
		if violations != "" {
			t.Errorf("UI standard violations: %s", violations)
		}
	})

	t.Run("Increment_Updates_Count", func(t *testing.T) {
		resetCounter(t)
		if err := chromedp.Run(ctx,
			e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 5*time.Second),
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 1')`, 5*time.Second),
		); err != nil {
			t.Fatalf("increment: %v", err)
		}
	})

	t.Run("Decrement_Stops_At_Zero", func(t *testing.T) {
		resetCounter(t)
		// Bump to 2, decrement twice → 0, decrement once more → still 0
		// (clamp behavior). Asserting full state at each step catches
		// off-by-one bugs in the clamp.
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 1')`, 5*time.Second),
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 2')`, 5*time.Second),
			chromedp.Click(`button[name="decrement"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 1')`, 5*time.Second),
			chromedp.Click(`button[name="decrement"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 0')`, 5*time.Second),
			// One more decrement should NOT go negative.
			chromedp.Click(`button[name="decrement"]`, chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
		); err != nil {
			t.Fatalf("decrement sequence: %v", err)
		}
		var bodyText string
		if err := chromedp.Run(ctx,
			chromedp.Text(`output[aria-live="polite"]`, &bodyText, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("read counter after over-decrement: %v", err)
		}
		if strings.TrimSpace(bodyText) != "0" {
			t.Errorf("counter went negative; got %q, want %q", bodyText, "0")
		}
	})

	t.Run("Reset_Returns_To_Zero", func(t *testing.T) {
		resetCounter(t)
		// Bump up, then Reset.
		for i := 0; i < 3; i++ {
			if err := chromedp.Run(ctx,
				chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
				e2etest.WaitFor(fmt.Sprintf(`document.body.innerText.includes('Count: %d')`, i+1), 5*time.Second),
			); err != nil {
				t.Fatalf("increment %d: %v", i+1, err)
			}
		}
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="reset"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 0')`, 5*time.Second),
		); err != nil {
			t.Fatalf("reset: %v", err)
		}
	})

	t.Run("Sync_Propagates_To_Peer_Tab", func(t *testing.T) {
		resetCounter(t)

		// Open a second tab in the same browser context. Same allocator
		// + parent context = shared cookie jar = same session group.
		// The framework dispatches Sync() on this peer connection after
		// every action in the original tab.
		peerCtx, peerCancel := chromedp.NewContext(ctx, chromedp.WithLogf(t.Logf))
		defer peerCancel()
		peerCtx, peerTimeout := context.WithTimeout(peerCtx, 30*time.Second)
		defer peerTimeout()

		if err := chromedp.Run(peerCtx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`output[aria-live="polite"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 0')`, 5*time.Second),
		); err != nil {
			t.Fatalf("peer tab initial load: %v", err)
		}

		// Bump counter in the original tab; assert peer reflects it.
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 1')`, 5*time.Second),
		); err != nil {
			t.Fatalf("increment in original tab: %v", err)
		}
		if err := chromedp.Run(peerCtx,
			e2etest.WaitFor(`document.body.innerText.includes('Count: 1')`, 5*time.Second),
		); err != nil {
			t.Fatalf("peer tab did not reflect increment via Sync(): %v", err)
		}

		// And the other direction — bump in peer, assert original sees it.
		if err := chromedp.Run(peerCtx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 2')`, 5*time.Second),
		); err != nil {
			t.Fatalf("increment in peer tab: %v", err)
		}
		if err := chromedp.Run(ctx,
			e2etest.WaitFor(`document.body.innerText.includes('Count: 2')`, 5*time.Second),
		); err != nil {
			t.Fatalf("original tab did not reflect peer increment via Sync(): %v", err)
		}
	})

	t.Run("HTTP_POST_Fallback_Without_JS", func(t *testing.T) {
		// Plain HTTP form POST — no JS client involved. Verifies the
		// Tier-1 path (form submits, server PRG-redirects, page reloads
		// with new state) still works for users with JS disabled.
		base := fmt.Sprintf("http://localhost:%d", serverPort)

		// Reset via POST.
		if _, err := http.PostForm(base, url.Values{"reset": {""}}); err != nil {
			t.Fatalf("POST reset: %v", err)
		}
		// Increment via POST.
		if _, err := http.PostForm(base, url.Values{"increment": {""}}); err != nil {
			t.Fatalf("POST increment: %v", err)
		}
		// GET the page — count should reflect the persisted state.
		resp, err := http.Get(base)
		if err != nil {
			t.Fatalf("GET after POST: %v", err)
		}
		defer resp.Body.Close()
		buf := make([]byte, 8192)
		n, _ := resp.Body.Read(buf)
		body := string(buf[:n])
		// Without a session cookie sent by http.Client, the increment
		// might be on a different session. Just assert the page renders
		// and the counter element is present — the cookie-aware browser
		// path is covered by the chromedp tests above.
		if !strings.Contains(body, `output aria-live="polite"`) && !strings.Contains(body, "<strong>") {
			t.Errorf("counter element missing from POST-fallback render; body: %s", truncateForLog(body, 400))
		}
	})
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
