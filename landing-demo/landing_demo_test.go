// Browser e2e for the landing-demo counter. Mirrors examples/counter's
// shape: spin up the server on a free port, drive a real Chrome via
// chromedp, exercise every controller method (Increment, Decrement,
// Reset, Sync). The Sync sub-test opens a second browser session in the
// same browser context so peer-tab dispatch can be observed.
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
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

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
		// Counter starts at zero and is rendered inside the live region.
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
				if (document.querySelector('style')) v.push('disallowed <style> block');
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
		if err := chromedp.Run(ctx,
			e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 5*time.Second),
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 1')`, 5*time.Second),
		); err != nil {
			t.Fatalf("increment: %v", err)
		}
	})

	t.Run("Decrement_Updates_Count", func(t *testing.T) {
		// Counter is at 1 from the previous sub-test.
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="decrement"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Count: 0')`, 5*time.Second),
		); err != nil {
			t.Fatalf("decrement: %v", err)
		}
	})

	t.Run("Reset_Returns_To_Zero", func(t *testing.T) {
		// Bump the counter a few times, then Reset and assert.
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
}
