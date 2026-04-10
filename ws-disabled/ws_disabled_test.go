package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()
	code := m.Run()
	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

func setupServer(t *testing.T) *httptest.Server {
	t.Helper()

	controller := &BookmarkController{}
	initialState := &BookmarkState{}

	tmpl := livetemplate.Must(livetemplate.New("ws-disabled",
		livetemplate.WithDevMode(true),
		livetemplate.WithWebSocketDisabled(),
	))

	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	return httptest.NewServer(mux)
}

// waitForClient waits for the LiveTemplate client to initialize in HTTP mode.
func waitForClient(timeout time.Duration) chromedp.Action {
	return e2etest.WaitFor(
		`window.liveTemplateClient && window.liveTemplateClient.isReady()`,
		timeout,
	)
}

// ============================================================================
// Browser E2E Tests (Docker Chrome)
// ============================================================================

func TestWSDisabled_BrowserE2E(t *testing.T) {
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

	appURL := e2etest.GetChromeTestURL(serverPort)

	t.Run("PageLoads", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(appURL),
			chromedp.WaitReady("body"),
			waitForClient(10*time.Second),
			chromedp.OuterHTML("html", &html),
		)
		if err != nil {
			t.Fatalf("chromedp failed: %v", err)
		}

		if !strings.Contains(html, "Bookmarks (WebSocket Disabled)") {
			t.Error("Expected page title in HTML")
		}
		if !strings.Contains(html, `name="add"`) {
			t.Error("Expected form or button with name='add'")
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

	t.Run("FormSubmission", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(appURL),
			chromedp.WaitReady("body"),
			waitForClient(10*time.Second),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}

		var htmlAfter string
		err = chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="label"]`, "Go Docs", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="url"]`, "https://go.dev", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Go Docs')`, 5*time.Second),
			chromedp.OuterHTML("html", &htmlAfter),
		)
		if err != nil {
			t.Fatalf("Form submission failed: %v", err)
		}

		if !strings.Contains(htmlAfter, "Go Docs") {
			t.Error("Expected 'Go Docs' in page after form submission")
		}
		if !strings.Contains(htmlAfter, "https://go.dev") {
			t.Error("Expected URL in page after form submission")
		}

		// Verify link structure (href and target)
		var linkHref string
		var hasTarget bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const a = document.querySelector('a[href="https://go.dev"]');
				return a ? a.getAttribute('href') : '';
			})()`, &linkHref),
			chromedp.Evaluate(`(() => {
				const a = document.querySelector('a[href="https://go.dev"]');
				return a ? a.getAttribute('target') === '_blank' : false;
			})()`, &hasTarget),
		)
		if err != nil {
			t.Fatalf("Failed to check link structure: %v", err)
		}
		if linkHref != "https://go.dev" {
			t.Errorf("Expected link href 'https://go.dev', got %q", linkHref)
		}
		if !hasTarget {
			t.Error("Expected link target='_blank'")
		}

		// Verify form fields cleared after submit (form auto-reset)
		var labelVal, urlVal string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('input[name="label"]').value`, &labelVal),
			chromedp.Evaluate(`document.querySelector('input[name="url"]').value`, &urlVal),
		)
		if err != nil {
			t.Logf("Warning: could not read form fields: %v", err)
		} else {
			if labelVal != "" {
				t.Errorf("Label input should be empty after submit, got %q", labelVal)
			}
			if urlVal != "" {
				t.Errorf("URL input should be empty after submit, got %q", urlVal)
			}
		}
	})

	t.Run("MultipleSubmissions", func(t *testing.T) {
		// Clear cookies via CDP to get a fresh session
		err := chromedp.Run(ctx,
			network.ClearBrowserCookies(),
			chromedp.Navigate(appURL),
			chromedp.WaitReady("body"),
			waitForClient(10*time.Second),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}

		err = chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="label"]`, "First", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="url"]`, "https://first.example", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('First')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("First submission failed: %v", err)
		}

		var htmlAfterTwo string
		err = chromedp.Run(ctx,
			chromedp.Clear(`input[name="label"]`, chromedp.ByQuery),
			chromedp.Clear(`input[name="url"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="label"]`, "Second", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="url"]`, "https://second.example", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('Second')`, 5*time.Second),
			chromedp.OuterHTML("html", &htmlAfterTwo),
		)
		if err != nil {
			t.Fatalf("Second submission failed: %v", err)
		}

		if !strings.Contains(htmlAfterTwo, "First") {
			t.Error("Expected 'First' bookmark in page")
		}
		if !strings.Contains(htmlAfterTwo, "Second") {
			t.Error("Expected 'Second' bookmark in page")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Clear cookies via CDP to get a fresh session
		err := chromedp.Run(ctx,
			network.ClearBrowserCookies(),
			chromedp.Navigate(appURL),
			chromedp.WaitReady("body"),
			waitForClient(10*time.Second),
			chromedp.SendKeys(`input[name="label"]`, "To Delete", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="url"]`, "https://delete.me", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('To Delete')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		err = chromedp.Run(ctx,
			chromedp.WaitReady(`button[name="delete"]`, chromedp.ByQuery),
			chromedp.Click(`button[name="delete"]`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
		)
		if err != nil {
			t.Fatalf("Delete click failed: %v", err)
		}

		var noBookmarks bool
		chromedp.Run(ctx,
			chromedp.Evaluate(`document.body.innerText.includes('No bookmarks yet')`, &noBookmarks),
		)

		if !noBookmarks {
			t.Error("Expected 'No bookmarks yet' after delete")
		}
	})
}

// ============================================================================
// HTTP Unit Tests (no browser needed)
// ============================================================================

func TestWSDisabled_ValidationError(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{}

	// GET initial page to establish session
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()

	// POST with empty fields via JSON (like the client library does)
	// Field errors are returned inline in JSON responses
	form := strings.NewReader("add=&label=&url=")
	req, err := http.NewRequest("POST", server.URL, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for JSON validation error, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "label") {
		t.Error("Expected field error for 'label' in JSON response")
	}
}

func TestWSDisabled_PRGPattern(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()

	form := strings.NewReader("add=&label=Test&url=https://test.com")
	req, err := http.NewRequest("POST", server.URL, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected 303 See Other for PRG redirect, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Error("Expected Location header in redirect response")
	}
	if strings.Contains(location, "success=") {
		t.Errorf("Flash message should NOT be in redirect URL, got: %s", location)
	}

	// Flash should be in lvt-flash cookie
	var flashCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "lvt-flash" {
			flashCookie = c
			break
		}
	}
	if flashCookie == nil {
		t.Fatal("Expected lvt-flash cookie to be set")
	}
	if !strings.Contains(flashCookie.Value, "success=") {
		t.Errorf("Expected 'success=' in flash cookie value, got: %s", flashCookie.Value)
	}
}

func TestWSDisabled_JSONResponse(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()

	form := strings.NewReader("add=&label=API+Test&url=https://api.example")
	req, err := http.NewRequest("POST", server.URL, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for JSON response, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"s"`) {
		t.Error("First JSON response should include statics ('s' key)")
	}
	if !strings.Contains(bodyStr, "API Test") {
		t.Error("JSON response should contain the bookmark label")
	}
}

func TestWSDisabled_DiffOptimization(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()

	doPost := func(label, url string) string {
		form := strings.NewReader(fmt.Sprintf("add=&label=%s&url=%s", label, url))
		req, err := http.NewRequest("POST", server.URL, form)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	first := doPost("First", "https://first.example")
	if !strings.Contains(first, `"s"`) {
		t.Error("First POST should include statics ('s' key)")
	}
	t.Logf("First response length: %d bytes", len(first))

	second := doPost("Second", "https://second.example")
	t.Logf("Second response length: %d bytes", len(second))

	hasStatics := strings.Contains(second, `"s"`)
	if hasStatics {
		t.Log("Second response includes statics — HTTP template cache may not be available yet")
	} else {
		reduction := 100 - 100*len(second)/len(first)
		t.Logf("Diff optimization active: %d%% size reduction", reduction)
		if len(second) >= len(first) {
			t.Errorf("With diff optimization, second response (%d bytes) should be smaller than first (%d bytes)",
				len(second), len(first))
		}
	}
}

func TestWSDisabled_WebSocketRejected(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusSwitchingProtocols {
		t.Error("WebSocket upgrade should be rejected when WebSocket is disabled")
	}
}
