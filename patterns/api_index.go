package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// apiCategory and apiPattern are the JSON shapes the docs site catalog
// consumes from /api/index.json. Kept separate from PatternLink so the
// internal model can evolve without breaking the public schema.
type apiCategory struct {
	Slug     string       `json:"slug"`
	Name     string       `json:"name"`
	Patterns []apiPattern `json:"patterns"`
}

type apiPattern struct {
	Slug        string `json:"slug"`        // last URL segment, e.g. "click-to-edit"
	Name        string `json:"name"`        // human title
	Path        string `json:"path"`        // upstream path, e.g. "/patterns/forms/click-to-edit"
	Description string `json:"description"` // one-line problem statement
	Status      string `json:"status"`      // "stable" | "soon"
	Category    string `json:"category"`    // human category name (denormalized for client convenience)
}

// apiIndexHandler exposes the pattern catalog as JSON for the
// LiveTemplate docs site (and any other consumer that wants to embed
// the catalog without scraping HTML).
//
// Response shape:
//
//	{
//	  "version": 1,
//	  "categories": [{slug, name, patterns: [{slug, name, path, description, status, category}]}]
//	}
//
// Versioned for forward-compat: bump `version` when the schema changes
// in a way that breaks existing consumers.
func apiIndexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS preflight first: a browser fetch from the docs site will
		// OPTIONS-preflight on any cross-origin request that the spec
		// considers non-simple. Reply with the same Allow-Origin we'd
		// send for the actual GET so the real request goes through.
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		categories := allPatterns()
		out := struct {
			Version    int           `json:"version"`
			Categories []apiCategory `json:"categories"`
		}{
			Version:    1,
			Categories: make([]apiCategory, 0, len(categories)),
		}
		for _, c := range categories {
			ac := apiCategory{
				Slug:     categorySlug(c.Name),
				Name:     c.Name,
				Patterns: make([]apiPattern, 0, len(c.Patterns)),
			}
			for _, p := range c.Patterns {
				status := "stable"
				if !p.Implemented {
					status = "soon"
				}
				ac.Patterns = append(ac.Patterns, apiPattern{
					Slug:        patternSlugFromPath(p.Path),
					Name:        p.Name,
					Path:        p.Path,
					Description: p.Description,
					Status:      status,
					Category:    c.Name,
				})
			}
			out.Categories = append(out.Categories, ac)
		}

		w.Header().Set("Content-Type", "application/json")
		// Cached aggressively at fly's edge; this index is authoritatively
		// produced from compiled-in code so it only changes on redeploy.
		w.Header().Set("Cache-Control", "public, max-age=300")
		// Allow the docs site (and any operator script) to fetch this
		// from a different origin without a separate CORS proxy.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(out)
	})
}

// categorySlug derives a URL-safe slug from a category name. e.g.
// "Forms & Editing" -> "forms-editing". Used as a stable id the docs
// catalog can address without depending on the human display name.
func categorySlug(name string) string {
	out := strings.ToLower(name)
	out = strings.ReplaceAll(out, "&", "")
	// collapse runs of non-alnum to a single dash
	var b strings.Builder
	prevDash := true
	for _, r := range out {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// patternSlugFromPath extracts the pattern's last URL segment.
// "/patterns/forms/click-to-edit" -> "click-to-edit".
func patternSlugFromPath(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
