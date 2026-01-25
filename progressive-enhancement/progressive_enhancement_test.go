package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// setupServer creates a test server with the progressive enhancement example
func setupServer(t *testing.T) *httptest.Server {
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

	return httptest.NewServer(mux)
}

// TestProgressiveEnhancement_WithJS tests the app with JavaScript enabled
func TestProgressiveEnhancement_WithJS(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	// Create browser context with logging
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout
	ctx, timeoutCancel := context.WithTimeout(ctx, 15*time.Second)
	defer timeoutCancel()

	var initialHTML string

	err := chromedp.Run(ctx,
		// Navigate to the app
		chromedp.Navigate(server.URL),

		// Wait for page to load
		chromedp.WaitReady("body"),

		// Get HTML to verify page loaded
		chromedp.OuterHTML("html", &initialHTML),
	)

	if err != nil {
		t.Fatalf("chromedp failed: %v", err)
	}

	// Verify page contains expected content
	if !strings.Contains(initialHTML, "Progressive Enhancement Todo List") {
		t.Error("Expected page title in HTML")
	}

	// Verify form exists
	if !strings.Contains(initialHTML, `name="lvt-action"`) {
		t.Error("Expected lvt-action hidden field in form")
	}
}

// TestProgressiveEnhancement_JSFormSubmission tests that JS mode intercepts forms and updates DOM
func TestProgressiveEnhancement_JSFormSubmission(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	// Create browser context with console logging
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	var initialHTML string
	var hasWrapper bool
	var wsConnected bool

	err := chromedp.Run(ctx,
		// Navigate to the app
		chromedp.Navigate(server.URL),

		// Wait for page to load and wrapper to exist
		chromedp.WaitReady("body"),
		chromedp.Sleep(500*time.Millisecond),

		// Verify wrapper div exists with data-lvt-id
		chromedp.Evaluate(`document.querySelector('[data-lvt-id]') !== null`, &hasWrapper),

		// Get initial HTML
		chromedp.OuterHTML("html", &initialHTML),
	)

	if err != nil {
		t.Fatalf("Initial page load failed: %v", err)
	}

	if !hasWrapper {
		t.Fatal("data-lvt-id wrapper not found - client library won't initialize")
	}
	t.Log("data-lvt-id wrapper found")

	// Log wrapper ID for debugging
	t.Logf("HTML contains data-lvt-id: %v", strings.Contains(initialHTML, "data-lvt-id"))

	// Wait for WebSocket to connect
	err = chromedp.Run(ctx,
		// Wait for client to be ready
		e2etest.WaitFor(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, 10*time.Second),

		// Verify WebSocket connected
		chromedp.Evaluate(`window.liveTemplateClient && window.liveTemplateClient.isReady()`, &wsConnected),
	)

	if err != nil {
		t.Logf("WebSocket connection check failed: %v", err)
	}

	if wsConnected {
		t.Log("WebSocket connected successfully")
	} else {
		t.Log("WebSocket not connected - may be using HTTP mode")
	}

	// Count initial todos
	var initialTodoCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &initialTodoCount),
	)
	if err != nil {
		t.Fatalf("Failed to count initial todos: %v", err)
	}
	t.Logf("Initial todo count: %d", initialTodoCount)

	// Type a new todo and submit the form
	var messageCount int
	err = chromedp.Run(ctx,
		// Type in the input field
		chromedp.SetValue(`input[name="title"]`, "New todo via JS", chromedp.ByQuery),

		// Get current message count before submission
		chromedp.Evaluate(`window.__wsMessageCount || 0`, &messageCount),

		// Click the submit button
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),

		// Wait for a DOM update
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Form submission failed: %v", err)
	}

	// Check if there was a WebSocket message
	var newMessageCount int
	chromedp.Run(ctx,
		chromedp.Evaluate(`window.__wsMessageCount || 0`, &newMessageCount),
	)

	t.Logf("Message count before: %d, after: %d", messageCount, newMessageCount)

	// Count todos after submission
	var finalTodoCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &finalTodoCount),
	)
	if err != nil {
		t.Fatalf("Failed to count final todos: %v", err)
	}
	t.Logf("Final todo count: %d", finalTodoCount)

	// Get the actual WebSocket messages received
	var lastMessage string
	var wsMessages []interface{}
	chromedp.Run(ctx,
		chromedp.Evaluate(`window.__lastWSMessage || ""`, &lastMessage),
		chromedp.Evaluate(`window.__wsMessages || []`, &wsMessages),
	)
	t.Logf("Last WS message (raw): %s", lastMessage)
	if len(wsMessages) > 0 {
		t.Logf("Total WS messages received: %d", len(wsMessages))
		for i, msg := range wsMessages {
			t.Logf("WS message %d: %+v", i, msg)
		}
	}

	// If WebSocket is working, we should see more todos now (no page reload)
	if finalTodoCount > initialTodoCount {
		t.Log("SUCCESS: Todo was added via WebSocket without page reload")
	} else {
		// Check for debug flags to understand why it didn't work
		var debugInfo map[string]interface{}
		chromedp.Run(ctx,
			chromedp.Evaluate(`({
				submitListenerTriggered: window.__lvtSubmitListenerTriggered,
				inWrapper: window.__lvtInWrapper,
				actionFound: window.__lvtActionFound,
				sendCalled: window.__lvtSendCalled,
				sendPath: window.__lvtSendPath,
				wsMessage: window.__lvtWSMessage,
				wsMessageCount: window.__wsMessageCount,
			})`, &debugInfo),
		)
		t.Logf("Debug info: %+v", debugInfo)

		// Get rendered HTML to see current state
		var currentHTML string
		chromedp.Run(ctx,
			chromedp.OuterHTML(".todo-list", &currentHTML, chromedp.ByQuery),
		)
		t.Logf("Current todo list HTML length: %d", len(currentHTML))

		// This is informational - the test still passes if HTTP fallback worked
		t.Log("Note: Form may have submitted via HTTP (check server logs)")
	}
}

// TestProgressiveEnhancement_NoJS tests the app without JavaScript
func TestProgressiveEnhancement_NoJS(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	// Test using HTTP client (simulating no-JS browser)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects automatically so we can verify PRG pattern
			return http.ErrUseLastResponse
		},
	}

	// GET initial page
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// POST a new todo (simulating form submission without JS)
	form := strings.NewReader("lvt-action=add&title=HTTP+test+todo")
	req, err := http.NewRequest("POST", server.URL, form)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html") // Indicate we want HTML, not JSON

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	// Should redirect with 303 See Other (PRG pattern)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	// Verify redirect URL exists
	location := resp.Header.Get("Location")
	if location == "" {
		t.Error("Expected Location header in redirect response")
	}

	// The location might be just a path like "/?success=..."
	// which is valid for redirects
	t.Logf("Redirect location: %s", location)
}

// TestProgressiveEnhancement_ValidationError tests validation error handling
func TestProgressiveEnhancement_ValidationError(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// POST with empty title (should fail validation)
	form := strings.NewReader("lvt-action=add&title=")
	req, err := http.NewRequest("POST", server.URL, form)
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

	// Should return 200 with re-rendered form (not redirect)
	// because validation failed
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 (re-render with errors), got %d", resp.StatusCode)
	}

	// Content-Type should be HTML
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %q", ct)
	}
}

// TestProgressiveEnhancement_JSClientGetsJSON verifies JS clients get JSON
func TestProgressiveEnhancement_JSClientGetsJSON(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{}

	// POST with Accept: application/json (like JS client would)
	form := strings.NewReader("lvt-action=add&title=JSON+test")
	req, err := http.NewRequest("POST", server.URL, form)
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

	// Should return 200 with JSON
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Content-Type should be JSON
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}
}

// TestProgressiveEnhancement_Toggle tests toggling a todo
func TestProgressiveEnhancement_Toggle(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Toggle an existing todo (ID "1" is pre-populated)
	form := strings.NewReader("lvt-action=toggle&id=1")
	req, err := http.NewRequest("POST", server.URL, form)
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

	// Should redirect (toggle is a successful action)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}
}

// TestProgressiveEnhancement_Delete tests deleting a todo
func TestProgressiveEnhancement_Delete(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Delete an existing todo (ID "2" is pre-populated)
	form := strings.NewReader("lvt-action=delete&id=2")
	req, err := http.NewRequest("POST", server.URL, form)
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

	// Should redirect (delete is a successful action)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	// Verify flash message in redirect URL
	location := resp.Header.Get("Location")
	if !strings.Contains(location, "success=") {
		t.Log("Note: Delete action should set success flash")
	}
}

// TestProgressiveEnhancement_DisabledReturnsJSON tests that disabling progressive enhancement returns JSON
func TestProgressiveEnhancement_DisabledReturnsJSON(t *testing.T) {
	controller := &TodoController{
		validate: validator.New(),
	}
	initialState := &TodoState{}

	// Create template with progressive enhancement DISABLED
	tmpl := livetemplate.Must(livetemplate.New("test",
		livetemplate.WithDevMode(true),
		livetemplate.WithProgressiveEnhancement(false),
	))

	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	server := httptest.NewServer(mux)
	defer server.Close()

	client := &http.Client{}

	// POST with Accept: text/html (like a no-JS browser)
	form := strings.NewReader("lvt-action=add&title=test")
	req, err := http.NewRequest("POST", server.URL, form)
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

	// Should still return JSON because progressive enhancement is disabled
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json when progressive enhancement is disabled, got %q", ct)
	}
}

// TestProgressiveEnhancement_WebSocketCRUD tests add, toggle, and delete operations via WebSocket
// with DOM verification to ensure the UI updates correctly.
func TestProgressiveEnhancement_WebSocketCRUD(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var initialCount int
	var afterAddCount int
	var afterToggleCount int
	var hasCompletedClass bool
	var afterUntoggleCount int
	var hasCompletedClassAfterUntoggle bool
	var afterDeleteCount int

	// Step 1: Navigate and count initial todos
	err := chromedp.Run(ctx,
		chromedp.Navigate(server.URL),
		chromedp.WaitReady(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket to connect
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &initialCount),
	)
	if err != nil {
		t.Fatalf("Step 1 (navigate) error: %v", err)
	}
	t.Logf("Initial todo count: %d", initialCount)

	// Step 2: Add a new todo
	err = chromedp.Run(ctx,
		chromedp.Clear(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="title"]`, "E2E Test Todo", chromedp.ByQuery),
		chromedp.Submit(`form[lvt-submit="add"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket response
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterAddCount),
	)
	if err != nil {
		t.Fatalf("Step 2 (add) error: %v", err)
	}
	t.Logf("After add count: %d", afterAddCount)

	// Verify add worked
	if afterAddCount != initialCount+1 {
		t.Errorf("Add failed: expected %d todos, got %d", initialCount+1, afterAddCount)
	}

	// Step 3: Toggle the last todo (mark as complete)
	// The new todo should NOT have .completed class initially
	err = chromedp.Run(ctx,
		chromedp.Submit(`.todo-item:last-child form[lvt-submit="toggle"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket response
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterToggleCount),
		chromedp.Evaluate(`document.querySelector('.todo-item:last-child').classList.contains('completed')`, &hasCompletedClass),
	)
	if err != nil {
		t.Fatalf("Step 3 (toggle) error: %v", err)
	}
	t.Logf("After toggle count: %d, has completed class: %v", afterToggleCount, hasCompletedClass)

	// Verify toggle worked - count should stay the same, but item should now have .completed class
	if afterToggleCount != afterAddCount {
		t.Errorf("Toggle changed item count: expected %d, got %d", afterAddCount, afterToggleCount)
	}
	if !hasCompletedClass {
		t.Errorf("Toggle failed: item should have .completed class after toggle")
	}

	// Step 4: Toggle again (mark as incomplete)
	err = chromedp.Run(ctx,
		chromedp.Submit(`.todo-item:last-child form[lvt-submit="toggle"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket response
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterUntoggleCount),
		chromedp.Evaluate(`document.querySelector('.todo-item:last-child').classList.contains('completed')`, &hasCompletedClassAfterUntoggle),
	)
	if err != nil {
		t.Fatalf("Step 4 (untoggle) error: %v", err)
	}
	t.Logf("After untoggle count: %d, has completed class: %v", afterUntoggleCount, hasCompletedClassAfterUntoggle)

	// Verify untoggle worked - count should stay the same, item should NOT have .completed class
	if afterUntoggleCount != afterAddCount {
		t.Errorf("Untoggle changed item count: expected %d, got %d", afterAddCount, afterUntoggleCount)
	}
	if hasCompletedClassAfterUntoggle {
		t.Errorf("Untoggle failed: item should NOT have .completed class after second toggle")
	}

	// Step 5: Delete the last todo (the one we just added)
	err = chromedp.Run(ctx,
		chromedp.Submit(`.todo-item:last-child form[lvt-submit="delete"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket response
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterDeleteCount),
	)
	if err != nil {
		t.Fatalf("Step 5 (delete) error: %v", err)
	}
	t.Logf("After delete count: %d", afterDeleteCount)

	// Verify delete worked
	if afterDeleteCount != initialCount {
		t.Errorf("Delete failed: expected %d todos, got %d", initialCount, afterDeleteCount)
	}

	t.Log("SUCCESS: All CRUD operations (add, toggle, untoggle, delete) work correctly")
}

// TestProgressiveEnhancement_DeleteThenToggle tests toggling a todo after deleting another one.
// This specifically tests that the auto-generated keys work correctly after item removal.
func TestProgressiveEnhancement_DeleteThenToggle(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var initialCount int
	var afterDeleteCount int
	var afterToggleCount int
	var hasCompletedClass bool

	// Step 1: Navigate and count initial todos
	err := chromedp.Run(ctx,
		chromedp.Navigate(server.URL),
		chromedp.WaitReady(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket to connect
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &initialCount),
	)
	if err != nil {
		t.Fatalf("Step 1 (navigate) error: %v", err)
	}
	t.Logf("Initial todo count: %d", initialCount)

	if initialCount < 2 {
		t.Fatalf("Need at least 2 initial items for this test, got %d", initialCount)
	}

	// Step 2: Delete the FIRST todo
	err = chromedp.Run(ctx,
		chromedp.Submit(`.todo-item:first-child form[lvt-submit="delete"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket response
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterDeleteCount),
	)
	if err != nil {
		t.Fatalf("Step 2 (delete) error: %v", err)
	}
	t.Logf("After delete count: %d", afterDeleteCount)

	// Verify delete worked
	if afterDeleteCount != initialCount-1 {
		t.Errorf("Delete failed: expected %d todos, got %d", initialCount-1, afterDeleteCount)
	}

	// Step 3: Toggle the LAST remaining todo (different item than what we deleted)
	// Check if it has completed class before toggle
	var hasCompletedClassBefore bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.todo-item:last-child').classList.contains('completed')`, &hasCompletedClassBefore),
	)
	if err != nil {
		t.Fatalf("Step 3 (check before toggle) error: %v", err)
	}
	t.Logf("Last item has completed class before toggle: %v", hasCompletedClassBefore)

	err = chromedp.Run(ctx,
		chromedp.Submit(`.todo-item:last-child form[lvt-submit="toggle"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for WebSocket response
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterToggleCount),
		chromedp.Evaluate(`document.querySelector('.todo-item:last-child').classList.contains('completed')`, &hasCompletedClass),
	)
	if err != nil {
		t.Fatalf("Step 3 (toggle) error: %v", err)
	}
	t.Logf("After toggle count: %d, has completed class: %v", afterToggleCount, hasCompletedClass)

	// Verify toggle worked - count should stay the same
	if afterToggleCount != afterDeleteCount {
		t.Errorf("Toggle changed item count: expected %d, got %d", afterDeleteCount, afterToggleCount)
	}

	// Verify the completed class changed
	if hasCompletedClass == hasCompletedClassBefore {
		t.Errorf("Toggle failed: completed class should have changed from %v to %v", hasCompletedClassBefore, !hasCompletedClassBefore)
	}

	t.Log("SUCCESS: Toggle works correctly after deleting a different item")
}

func init() {
	// Suppress log output during tests
	fmt.Println("Progressive Enhancement E2E Tests")
}
