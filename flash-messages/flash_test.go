//go:build http

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate"
)

// TestFlash_ShowsInTemplate tests that flash messages appear in rendered HTML.
func TestFlash_ShowsInTemplate(t *testing.T) {
	// Create controller and state
	controller := &FlashController{}
	initialState := &FlashState{}

	// Create template
	tmpl := livetemplate.Must(livetemplate.New("flash",
		livetemplate.WithDevMode(true),
	))

	// Create test server
	handler := tmpl.Handle(controller, livetemplate.AsState(initialState))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create client with cookie jar for session persistence
	jar, _ := newCookieJar()
	client := &http.Client{Jar: jar}

	// 1. First GET to establish session and mount state
	resp, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("Initial GET failed: %v", err)
	}
	resp.Body.Close()

	// 2. POST to add item (should set success flash)
	form := url.Values{}
	form.Set("addItem", "")
	form.Set("item", "Test Item")

	resp, err = client.PostForm(server.URL+"/", form)
	if err != nil {
		t.Fatalf("POST add_item failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body := readBody(t, resp)

	// Flash message should appear in response
	if !strings.Contains(body, "Added item: Test Item") {
		t.Errorf("Expected success flash 'Added item: Test Item' in response, got:\n%s", body)
	}

	// Should have flash-success class
	if !strings.Contains(body, "flash-success") {
		t.Errorf("Expected flash-success class in response")
	}
}

// TestFlash_ClearsAfterAction tests that flash is cleared on subsequent action.
func TestFlash_ClearsAfterAction(t *testing.T) {
	controller := &FlashController{}
	initialState := &FlashState{}

	tmpl := livetemplate.Must(livetemplate.New("flash",
		livetemplate.WithDevMode(true),
	))

	handler := tmpl.Handle(controller, livetemplate.AsState(initialState))
	server := httptest.NewServer(handler)
	defer server.Close()

	jar, _ := newCookieJar()
	client := &http.Client{Jar: jar}

	// 1. GET to establish session
	resp, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("Initial GET failed: %v", err)
	}
	resp.Body.Close()

	// 2. POST to add first item (sets flash)
	form := url.Values{}
	form.Set("addItem", "")
	form.Set("item", "First Item")
	resp, err = client.PostForm(server.URL+"/", form)
	if err != nil {
		t.Fatalf("First POST failed: %v", err)
	}
	body1 := readBody(t, resp)
	resp.Body.Close()

	if !strings.Contains(body1, "Added item: First Item") {
		t.Error("First action should show flash")
	}

	// 3. POST to add second item (new flash replaces old)
	form = url.Values{}
	form.Set("addItem", "")
	form.Set("item", "Second Item")
	resp, err = client.PostForm(server.URL+"/", form)
	if err != nil {
		t.Fatalf("Second POST failed: %v", err)
	}
	body2 := readBody(t, resp)
	resp.Body.Close()

	// Old flash should be gone
	if strings.Contains(body2, "Added item: First Item") {
		t.Error("Old flash should be cleared after new action")
	}

	// New flash should be present
	if !strings.Contains(body2, "Added item: Second Item") {
		t.Error("New flash should appear")
	}
}

// TestFlash_DifferentTypes tests success, error, warning, and info flash types.
func TestFlash_DifferentTypes(t *testing.T) {
	controller := &FlashController{}
	initialState := &FlashState{}

	tmpl := livetemplate.Must(livetemplate.New("flash",
		livetemplate.WithDevMode(true),
	))

	handler := tmpl.Handle(controller, livetemplate.AsState(initialState))
	server := httptest.NewServer(handler)
	defer server.Close()

	jar, _ := newCookieJar()
	client := &http.Client{Jar: jar}

	// Initial GET
	resp, _ := client.Get(server.URL + "/")
	resp.Body.Close()

	tests := []struct {
		name      string
		action    string
		item      string
		wantClass string
		wantText  string
	}{
		{
			name:      "success flash",
			action:    "addItem",
			item:      "New Item",
			wantClass: "flash-success",
			wantText:  "Added item: New Item",
		},
		{
			name:      "warning flash (duplicate)",
			action:    "addItem",
			item:      "New Item", // Same item = duplicate
			wantClass: "flash-warning",
			wantText:  "Item already exists",
		},
		{
			name:      "info flash",
			action:    "removeItem",
			item:      "New Item",
			wantClass: "flash-info",
			wantText:  "Removed item: New Item",
		},
		{
			name:      "error flash",
			action:    "simulateError",
			item:      "",
			wantClass: "flash-error",
			wantText:  "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			form.Set(tt.action, "")
			if tt.item != "" {
				form.Set("item", tt.item)
			}

			resp, err := client.PostForm(server.URL+"/", form)
			if err != nil {
				t.Fatalf("POST failed: %v", err)
			}
			body := readBody(t, resp)
			resp.Body.Close()

			if !strings.Contains(body, tt.wantClass) {
				t.Errorf("Expected %s in response", tt.wantClass)
			}
			if !strings.Contains(body, tt.wantText) {
				t.Errorf("Expected '%s' in response, got:\n%s", tt.wantText, body)
			}
		})
	}
}

// TestFlash_FieldErrorsStillWork tests that field errors work alongside flash.
func TestFlash_FieldErrorsStillWork(t *testing.T) {
	controller := &FlashController{}
	initialState := &FlashState{}

	tmpl := livetemplate.Must(livetemplate.New("flash",
		livetemplate.WithDevMode(true),
	))

	handler := tmpl.Handle(controller, livetemplate.AsState(initialState))
	server := httptest.NewServer(handler)
	defer server.Close()

	jar, _ := newCookieJar()
	client := &http.Client{Jar: jar}

	// Initial GET
	resp, _ := client.Get(server.URL + "/")
	resp.Body.Close()

	// POST with empty item (triggers field error, not flash)
	form := url.Values{}
	form.Set("addItem", "")
	form.Set("item", "") // Empty = validation error

	resp, err := client.PostForm(server.URL+"/", form)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	body := readBody(t, resp)
	resp.Body.Close()

	// Should show field error, not flash
	if !strings.Contains(body, "Item name is required") {
		t.Error("Expected field error 'Item name is required'")
	}
	if !strings.Contains(body, "field-error") {
		t.Error("Expected field-error class")
	}
}

// Helper functions

func newCookieJar() (http.CookieJar, error) {
	// Simple cookie jar implementation
	return &simpleCookieJar{cookies: make(map[string][]*http.Cookie)}, nil
}

type simpleCookieJar struct {
	cookies map[string][]*http.Cookie
}

func (j *simpleCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.cookies[u.Host] = cookies
}

func (j *simpleCookieJar) Cookies(u *url.URL) []*http.Cookie {
	return j.cookies[u.Host]
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := new(strings.Builder)
	_, err := io.Copy(buf, resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	return buf.String()
}
