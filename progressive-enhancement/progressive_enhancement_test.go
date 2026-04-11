package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()

	code := m.Run()

	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

// setupServer creates a test server with the progressive enhancement example (for HTTP-only tests)
func setupServer(t *testing.T) *http.Server {
	t.Helper()

	controller := &TodoController{
		validate: validator.New(),
	}
	initialState := &TodoState{}

	tmpl := livetemplate.Must(livetemplate.New("progressive-enhancement",
		livetemplate.WithDevMode(true),
	))

	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	mux.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()

	// Wait for server to be ready
	addr := fmt.Sprintf("http://localhost:%d", port)
	for i := 0; i < 50; i++ {
		resp, err := http.Get(addr)
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Cleanup(func() { srv.Close() })
	return srv
}

func serverURL(srv *http.Server) string {
	return fmt.Sprintf("http://localhost%s", srv.Addr)
}

// ========== E2E Tests (Docker Chrome) ==========

func TestProgressiveEnhancementE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	srvPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	serverCmd := e2etest.StartTestServer(t, "main.go", srvPort)
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

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(srvPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`html`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		if !strings.Contains(html, "Progressive Enhancement") {
			t.Error("Page title not found")
		}
		if !strings.Contains(html, `name="add"`) {
			t.Error("Expected button with name='add' in form")
		}
		if !strings.Contains(html, "livetemplate-client.js") {
			t.Error("Script tag for livetemplate-client.js not found")
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
		if err := chromedp.Run(ctx, e2etest.ValidatePicoCSS()); err != nil {
			t.Errorf("Pico CSS check failed: %v", err)
		}
		t.Log("✅ UI standards passed")
	})

	t.Run("WebSocket_Connection", func(t *testing.T) {
		var wsConnected bool
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, &wsConnected),
		)
		if err != nil {
			t.Fatalf("Failed to check WebSocket: %v", err)
		}
		if !wsConnected {
			t.Error("WebSocket not connected")
		}
		t.Log("✅ WebSocket connection established")
	})

	t.Run("Add_Todo", func(t *testing.T) {
		var initialCount int
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &initialCount),
		)
		if err != nil {
			t.Fatalf("Failed to count todos: %v", err)
		}

		expectedCount := initialCount + 1
		err = chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="title"]`, "E2E Test Todo", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(fmt.Sprintf(`document.querySelectorAll('table tbody tr').length === %d`, expectedCount), 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add todo: %v", err)
		}

		var afterCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &afterCount),
		)
		if err != nil {
			t.Fatalf("Failed to count after add: %v", err)
		}
		if afterCount != expectedCount {
			t.Errorf("Expected %d todos, got %d", expectedCount, afterCount)
		}

		// Verify input was cleared (form auto-reset)
		var inputValue string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('input[name="title"]').value`, &inputValue),
		)
		if err != nil {
			t.Fatalf("Failed to get input value: %v", err)
		}
		if inputValue != "" {
			t.Errorf("Input should be cleared after add, got: %q", inputValue)
		}

		t.Logf("✅ Todo added, count: %d → %d, input cleared", initialCount, afterCount)
	})

	t.Run("Toggle_Todo", func(t *testing.T) {
		// Toggle the last todo (mark as complete)
		err := chromedp.Run(ctx,
			chromedp.Click(`table tbody tr:last-child button[name="toggle"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('table tbody tr:last-child').querySelector('s') !== null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to toggle todo: %v", err)
		}

		var hasStrikethrough bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('table tbody tr:last-child').querySelector('s') !== null`, &hasStrikethrough),
		)
		if err != nil {
			t.Fatalf("Failed to check strikethrough: %v", err)
		}
		if !hasStrikethrough {
			t.Error("Todo should have strikethrough after toggle")
		}

		// Toggle again (mark as incomplete)
		err = chromedp.Run(ctx,
			chromedp.Click(`table tbody tr:last-child button[name="toggle"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('table tbody tr:last-child').querySelector('s') === null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to untoggle todo: %v", err)
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('table tbody tr:last-child').querySelector('s') !== null`, &hasStrikethrough),
		)
		if err != nil {
			t.Fatalf("Failed to check strikethrough: %v", err)
		}
		if hasStrikethrough {
			t.Error("Todo should NOT have strikethrough after untoggle")
		}

		t.Log("✅ Toggle and untoggle work correctly")
	})

	t.Run("Delete_Todo", func(t *testing.T) {
		var beforeCount int
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &beforeCount),
		)
		if err != nil {
			t.Fatalf("Failed to count todos: %v", err)
		}

		expectedCount := beforeCount - 1
		err = chromedp.Run(ctx,
			chromedp.Click(`table tbody tr:last-child button[name="delete"]`, chromedp.ByQuery),
			e2etest.WaitFor(fmt.Sprintf(`document.querySelectorAll('table tbody tr').length === %d`, expectedCount), 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to delete todo: %v", err)
		}

		var afterCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &afterCount),
		)
		if err != nil {
			t.Fatalf("Failed to count after delete: %v", err)
		}
		if afterCount != expectedCount {
			t.Errorf("Expected %d todos after delete, got %d", expectedCount, afterCount)
		}

		t.Logf("✅ Todo deleted, count: %d → %d", beforeCount, afterCount)
	})

	t.Run("Delete_Then_Toggle", func(t *testing.T) {
		// Navigate fresh to get clean state (3 seed items)
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(srvPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to reload page: %v", err)
		}

		var initialCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &initialCount),
		)
		if err != nil {
			t.Fatalf("Failed to count todos: %v", err)
		}
		if initialCount < 2 {
			t.Fatalf("Need at least 2 items, got %d", initialCount)
		}

		// Delete the first todo
		expectedAfterDelete := initialCount - 1
		err = chromedp.Run(ctx,
			chromedp.Click(`table tbody tr:first-child button[name="delete"]`, chromedp.ByQuery),
			e2etest.WaitFor(fmt.Sprintf(`document.querySelectorAll('table tbody tr').length === %d`, expectedAfterDelete), 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to delete first item: %v", err)
		}

		// Toggle the last remaining todo
		var hadStrikethrough bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('table tbody tr:last-child').querySelector('s') !== null`, &hadStrikethrough),
		)
		if err != nil {
			t.Fatalf("Failed to check state before toggle: %v", err)
		}

		toggleWait := `document.querySelector('table tbody tr:last-child').querySelector('s') !== null`
		if hadStrikethrough {
			toggleWait = `document.querySelector('table tbody tr:last-child').querySelector('s') === null`
		}

		err = chromedp.Run(ctx,
			chromedp.Click(`table tbody tr:last-child button[name="toggle"]`, chromedp.ByQuery),
			e2etest.WaitFor(toggleWait, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to toggle after delete: %v", err)
		}

		var hasStrikethrough bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('table tbody tr:last-child').querySelector('s') !== null`, &hasStrikethrough),
		)
		if err != nil {
			t.Fatalf("Failed to check toggle result: %v", err)
		}
		if hasStrikethrough == hadStrikethrough {
			t.Errorf("Toggle failed: strikethrough should have changed from %v", hadStrikethrough)
		}

		t.Log("✅ Toggle works correctly after deleting a different item")
	})

	t.Run("Validation_Error_Browser", func(t *testing.T) {
		// Navigate fresh
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(srvPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to reload page: %v", err)
		}

		var beforeCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &beforeCount),
		)
		if err != nil {
			t.Fatalf("Failed to count todos: %v", err)
		}

		// Submit with a too-short title (less than 3 chars)
		err = chromedp.Run(ctx,
			chromedp.SetValue(`input[name="title"]`, "ab", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('[aria-invalid="true"]') !== null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to submit invalid form: %v", err)
		}

		var afterCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &afterCount),
		)
		if err != nil {
			t.Fatalf("Failed to count after invalid submit: %v", err)
		}
		if afterCount != beforeCount {
			t.Errorf("Count should be unchanged after validation error: expected %d, got %d", beforeCount, afterCount)
		}

		t.Log("✅ Validation error handled correctly in browser")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("All progressive-enhancement E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}

// ========== HTTP-only Tests (no browser needed) ==========

func TestProgressiveEnhancement_NoJS(t *testing.T) {
	srv := setupServer(t)
	url := serverURL(srv)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// GET initial page
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// POST a new todo (simulating form submission without JS)
	form := strings.NewReader("add=&title=HTTP+test+todo")
	req, err := http.NewRequest("POST", url, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	// Should redirect with 303 See Other (PRG pattern)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Error("Expected Location header in redirect response")
	}
	t.Logf("Redirect location: %s", location)
}

func TestProgressiveEnhancement_ValidationError(t *testing.T) {
	srv := setupServer(t)
	url := serverURL(srv)
	client := &http.Client{}

	// GET to establish session
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	cookies := resp.Cookies()
	resp.Body.Close()

	// POST with empty title via JSON
	form := strings.NewReader("add=&title=")
	req, err := http.NewRequest("POST", url, form)
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
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}
}

func TestProgressiveEnhancement_JSClientGetsJSON(t *testing.T) {
	srv := setupServer(t)
	url := serverURL(srv)
	client := &http.Client{}

	form := strings.NewReader("add=&title=JSON+test")
	req, err := http.NewRequest("POST", url, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}
}

func TestProgressiveEnhancement_HTTPToggle(t *testing.T) {
	srv := setupServer(t)
	url := serverURL(srv)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := strings.NewReader("toggle=&id=1")
	req, err := http.NewRequest("POST", url, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}
}

func TestProgressiveEnhancement_HTTPDelete(t *testing.T) {
	srv := setupServer(t)
	url := serverURL(srv)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := strings.NewReader("delete=&id=2")
	req, err := http.NewRequest("POST", url, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}
}

func TestProgressiveEnhancement_DisabledReturnsJSON(t *testing.T) {
	controller := &TodoController{
		validate: validator.New(),
	}
	initialState := &TodoState{}

	tmpl := livetemplate.Must(livetemplate.New("test",
		livetemplate.WithDevMode(true),
		livetemplate.WithProgressiveEnhancement(false),
	))

	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	defer srv.Close()

	url := fmt.Sprintf("http://localhost:%d", port)
	// Wait for server
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	client := &http.Client{}
	form := strings.NewReader("add=&title=test")
	req, err := http.NewRequest("POST", url, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json when progressive enhancement is disabled, got %q", ct)
	}
}
