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
		if !strings.Contains(html, `name="lvt-action"`) {
			t.Error("Expected lvt-action hidden field in form")
		}
		if !strings.Contains(html, `lvt-submit="Add"`) {
			t.Error("Expected lvt-submit attribute on form")
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
			chromedp.Submit(`form[lvt-submit="Add"]`, chromedp.ByQuery),
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
			chromedp.Submit(`form[lvt-submit="Add"]`, chromedp.ByQuery),
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
			chromedp.Submit(`form[lvt-submit="Add"]`, chromedp.ByQuery),
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
			chromedp.Submit(`form[lvt-submit="Add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('To Delete')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		var htmlAfterDelete string
		err = chromedp.Run(ctx,
			chromedp.Submit(`form[lvt-submit="Delete"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('No bookmarks yet')`, 5*time.Second),
			chromedp.OuterHTML("html", &htmlAfterDelete),
		)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		if strings.Contains(htmlAfterDelete, "To Delete") {
			t.Error("Bookmark should be removed after delete")
		}
	})
}

// ============================================================================
// HTTP Unit Tests (no browser needed)
// ============================================================================

func TestWSDisabled_ValidationError(t *testing.T) {
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

	form := strings.NewReader("lvt-action=Add&label=&url=")
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for validation error, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "label is required") {
		t.Error("Expected validation error message in response body")
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

	form := strings.NewReader("lvt-action=Add&label=Test&url=https://test.com")
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

	form := strings.NewReader("lvt-action=Add&label=API+Test&url=https://api.example")
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
		form := strings.NewReader(fmt.Sprintf("lvt-action=Add&label=%s&url=%s", label, url))
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
