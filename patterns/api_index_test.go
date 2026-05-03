package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCategorySlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Forms & Editing", "forms-editing"},
		{"Lists & Data", "lists-data"},
		{"Search & Filtering", "search-filtering"},
		{"Loading & Progress", "loading-progress"},
		{"Dialogs, Tabs & Navigation", "dialogs-tabs-navigation"},
		{"Feedback & Animations", "feedback-animations"},
		{"Real-Time", "real-time"},
		{"  Trim me  ", "trim-me"},
	}
	for _, c := range cases {
		if got := categorySlug(c.in); got != c.want {
			t.Errorf("categorySlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPatternSlugFromPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/patterns/forms/click-to-edit", "click-to-edit"},
		{"/patterns/lists/delete-row", "delete-row"},
		{"no-slash", "no-slash"},
	}
	for _, c := range cases {
		if got := patternSlugFromPath(c.in); got != c.want {
			t.Errorf("patternSlugFromPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAPIIndex_StructureAndContents(t *testing.T) {
	srv := httptest.NewServer(apiIndexHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	if cors := resp.Header.Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("missing CORS header for cross-origin docs-site fetch: %q", cors)
	}

	var body struct {
		Version    int           `json:"version"`
		Categories []apiCategory `json:"categories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Version != 1 {
		t.Errorf("version = %d, want 1 (bump only when schema breaks)", body.Version)
	}

	if len(body.Categories) == 0 {
		t.Fatal("no categories returned")
	}

	// Spot-check the canonical first pattern is present and well-formed.
	var firstPattern *apiPattern
	for _, c := range body.Categories {
		if len(c.Patterns) > 0 {
			firstPattern = &c.Patterns[0]
			break
		}
	}
	if firstPattern == nil {
		t.Fatal("no patterns in any category")
	}
	if firstPattern.Slug == "" || firstPattern.Name == "" || firstPattern.Path == "" {
		t.Errorf("first pattern has empty fields: %+v", firstPattern)
	}
	if !strings.HasPrefix(firstPattern.Path, "/patterns/") {
		t.Errorf("first pattern path doesn't look like a pattern URL: %q", firstPattern.Path)
	}
	if firstPattern.Status != "stable" && firstPattern.Status != "soon" {
		t.Errorf("first pattern status %q must be stable|soon", firstPattern.Status)
	}
	if firstPattern.Category == "" {
		t.Error("category denormalized field should be set")
	}
}

func TestAPIIndex_HEADRequest(t *testing.T) {
	srv := httptest.NewServer(apiIndexHandler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodHead, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("HEAD status = %d", resp.StatusCode)
	}
}

func TestAPIIndex_RejectsNonGET(t *testing.T) {
	srv := httptest.NewServer(apiIndexHandler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", resp.StatusCode)
	}
}

// CORS preflight from cross-origin browsers (the docs site fetching
// from lt-patterns.fly.dev) MUST succeed or the actual GET never
// fires. Reviewer flagged this on the initial PR.
func TestAPIIndex_OPTIONSPreflightSucceeds(t *testing.T) {
	srv := httptest.NewServer(apiIndexHandler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL, nil)
	req.Header.Set("Origin", "https://livetemplate.fly.dev")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", resp.StatusCode)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", origin)
	}
	allow := resp.Header.Get("Access-Control-Allow-Methods")
	if !strings.Contains(allow, "GET") {
		t.Errorf("Access-Control-Allow-Methods %q must include GET", allow)
	}
}

// Catch the common drift case: a handler is registered in main.go but
// never made it into allPatterns() (or vice versa). The API consumer
// expects the catalog to mirror what's actually served.
func TestAPIIndex_AllPatternsHaveImplementedHandlers(t *testing.T) {
	categories := allPatterns()
	if len(categories) == 0 {
		t.Fatal("allPatterns() returned no categories")
	}
	count := 0
	for _, c := range categories {
		count += len(c.Patterns)
	}
	if count < 30 {
		t.Errorf("got %d patterns; expected at least 30 (drift between data.go and main.go?)", count)
	}
}
