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
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()
	code := m.Run()
	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

// startNotepadServer starts the shared-notepad server and captures its logs.
func startNotepadServer(t *testing.T) (int, *e2etest.SafeBuffer) {
	t.Helper()

	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	portStr := fmt.Sprintf("%d", port)
	cmd := exec.Command("go", "run", "main.go")
	// LVT_DEV_MODE=true so the spawned process uses the local client library
	cmd.Env = append(os.Environ(), "PORT="+portStr, "LVT_DEV_MODE=true", "GOWORK=off")

	logs := e2etest.NewSafeBuffer()
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		t.Logf("=== SERVER LOGS ===\n%s", logs.String())
	})

	// Wait for server
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for i := 0; i < 50; i++ {
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/livetemplate-client.js", port))
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return port, logs
}

func TestSharedNotepadE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	serverPort, serverLogs := startNotepadServer(t)

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

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

	appURL := fmt.Sprintf("http://alice:demo@host.docker.internal:%d", serverPort)

	t.Run("PageLoads", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(appURL),
			chromedp.WaitReady("body"),
			e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 10*time.Second),
			chromedp.OuterHTML("body", &html),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Shared Notepad") {
			t.Error("Page title not found")
		}
		if !strings.Contains(html, "alice") {
			t.Error("Username not displayed")
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
	})

	t.Run("TypeSaveAndRefresh", func(t *testing.T) {
		// Type in textarea
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`#content`, chromedp.ByQuery),
			chromedp.Click(`#content`, chromedp.ByQuery),
			chromedp.SendKeys(`#content`, "Hello persistence test", chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to type: %v", err)
		}

		// Verify text before save
		var beforeSave string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('content').value`, &beforeSave),
		)
		if err != nil {
			t.Fatalf("Failed to read textarea: %v", err)
		}
		t.Logf("Before save: %q", beforeSave)
		if !strings.Contains(beforeSave, "Hello persistence test") {
			t.Fatalf("Typed text not in textarea: %q", beforeSave)
		}

		// Verify character count updated
		var charCountText string
		err = chromedp.Run(ctx,
			chromedp.TextContent(`#charcount`, &charCountText, chromedp.ByQuery),
		)
		if err != nil {
			t.Logf("Warning: could not read charcount: %v", err)
		} else if !strings.Contains(charCountText, "characters") {
			t.Errorf("Char count should contain 'characters', got %q", charCountText)
		}

		// Click Save and wait for timestamp to appear (indicates save completed)
		err = chromedp.Run(ctx,
			chromedp.Click(`button[name="save"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('charcount').textContent.includes('saved at')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to save: %v", err)
		}

		// Verify after save
		var afterSave string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('content').value`, &afterSave),
		)
		if err != nil {
			t.Fatalf("Failed to read after save: %v", err)
		}
		t.Logf("After save: %q", afterSave)
		if afterSave == "" {
			t.Fatal("BUG: Textarea wiped after save")
		}

		// Verify timestamp appeared after save
		var charCountAfterSave string
		err = chromedp.Run(ctx,
			chromedp.TextContent(`#charcount`, &charCountAfterSave, chromedp.ByQuery),
		)
		if err != nil {
			t.Logf("Warning: could not read charcount after save: %v", err)
		} else if !strings.Contains(charCountAfterSave, "saved at") {
			t.Errorf("After save, charcount should contain 'saved at', got %q", charCountAfterSave)
		}

		// Log server state before refresh
		t.Logf("Server logs before refresh:\n%s", serverLogs.String())

		// Simulate browser favicon request (browsers do this on every page load)
		go func() {
			client := &http.Client{Timeout: 2 * time.Second}
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/favicon.ico", serverPort), nil)
			req.SetBasicAuth("alice", "demo")
			client.Do(req)
		}()
		time.Sleep(100 * time.Millisecond)

		// REFRESH the page
		err = chromedp.Run(ctx,
			chromedp.Navigate(appURL),
			chromedp.WaitReady("body"),
			e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to reload: %v", err)
		}

		// Check textarea after refresh
		var afterRefresh string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('content').value`, &afterRefresh),
		)
		if err != nil {
			t.Fatalf("Failed to read after refresh: %v", err)
		}
		t.Logf("After refresh: %q", afterRefresh)

		// Log server state after refresh
		t.Logf("Server logs after refresh:\n%s", serverLogs.String())

		if afterRefresh == "" {
			t.Fatal("BUG: Content lost after page refresh")
		}
		if !strings.Contains(afterRefresh, "Hello persistence test") {
			t.Errorf("Expected content to persist, got: %q", afterRefresh)
		}
	})
}
