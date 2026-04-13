package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Contact represents a person in the demo data.
type Contact struct {
	ID    string
	Name  string
	Email string
}

// UserRow represents a user with an active status toggle.
type UserRow struct {
	ID     string
	Name   string
	Email  string
	Active bool
}

// Item is a generic named item used across multiple patterns.
type Item struct {
	ID    string
	Name  string
	Email string
}

// FilterItem is a todo-like item with status and date, used by URL-Preserved Filters.
type FilterItem struct {
	ID     string
	Name   string
	Status string // "active" or "completed"
	Date   string // YYYY-MM-DD
}

func sampleContacts() []Contact {
	return []Contact{
		{ID: "1", Name: "Joe Smith", Email: "joe@smith.org"},
		{ID: "2", Name: "Angie MacDowell", Email: "angie@macdowell.org"},
		{ID: "3", Name: "Fuqua Tarkenton", Email: "fuqua@tarkenton.org"},
		{ID: "4", Name: "Kim Yee", Email: "kim@yee.org"},
	}
}

func sampleUsers() []UserRow {
	return []UserRow{
		{ID: "1", Name: "Joe Smith", Email: "joe@smith.org", Active: true},
		{ID: "2", Name: "Angie MacDowell", Email: "angie@macdowell.org", Active: true},
		{ID: "3", Name: "Fuqua Tarkenton", Email: "fuqua@tarkenton.org", Active: false},
		{ID: "4", Name: "Kim Yee", Email: "kim@yee.org", Active: false},
	}
}

// listDataset (25 items) is used by Delete Row and Click To Load.
// 25 at page size 10 gives three pages (10, 10, 5). Infinite Scroll uses
// a larger dedicated dataset so the auto-scroll cascade is actually visible.
var listDataset = buildItemDataset(25, "Item")

// infiniteScrollDataset (100 items) gives Infinite Scroll enough rows for
// the auto-pagination cascade to feel real under manual testing.
var infiniteScrollDataset = buildItemDataset(100, "Row")

func buildItemDataset(n int, namePrefix string) []Item {
	items := make([]Item, n)
	for i := range items {
		id := i + 1
		items[i] = Item{
			ID:    fmt.Sprintf("%d", id),
			Name:  fmt.Sprintf("%s %d", namePrefix, id),
			Email: fmt.Sprintf("%s%d@example.com", strings.ToLower(namePrefix), id),
		}
	}
	return items
}

// getItemPage returns a 1-indexed page of listDataset. Empty slice when
// out of range. Used by Click To Load and Delete Row's initial state.
func getItemPage(page, size int) []Item {
	return pageSlice(listDataset, page, size)
}

// getInfiniteScrollPage returns a 1-indexed page of infiniteScrollDataset.
func getInfiniteScrollPage(page, size int) []Item {
	return pageSlice(infiniteScrollDataset, page, size)
}

func pageSlice(dataset []Item, page, size int) []Item {
	if page < 1 || size < 1 {
		return nil
	}
	start := (page - 1) * size
	if start >= len(dataset) {
		return nil
	}
	end := start + size
	if end > len(dataset) {
		end = len(dataset)
	}
	return slices.Clone(dataset[start:end])
}

// carMakes maps car makes to their model lists. Used by Value Select to
// demonstrate cascading dependent selects.
var carMakes = map[string][]string{
	"Audi":   {"A3", "A4", "Q5", "R8"},
	"BMW":    {"3 Series", "5 Series", "X3", "M3"},
	"Toyota": {"Camry", "Corolla", "RAV4", "Highlander"},
}

// getCarMakes returns the sorted list of available car makes.
func getCarMakes() []string {
	return slices.Sorted(maps.Keys(carMakes))
}

// getCarModels returns a copy of the models for a given make, or nil if
// the make is unknown. Returning a copy defends callers from aliasing the
// package-level carMakes map.
func getCarModels(carMake string) []string {
	return slices.Clone(carMakes[carMake])
}

// contactDirectory is the 25-contact dataset used by Active Search. Kept
// distinct from sampleContacts() (4 entries) which is pinned by Edit Row tests.
var contactDirectory = []Contact{
	{ID: "1", Name: "Marcus Chen", Email: "marcus.chen@example.com"},
	{ID: "2", Name: "Priya Patel", Email: "priya.patel@example.com"},
	{ID: "3", Name: "Diana Okonkwo", Email: "diana.okonkwo@example.com"},
	{ID: "4", Name: "Rafael Hernandez", Email: "rafael.hernandez@example.com"},
	{ID: "5", Name: "Yuki Tanaka", Email: "yuki.tanaka@example.com"},
	{ID: "6", Name: "Fatima Al-Rashid", Email: "fatima.alrashid@example.com"},
	{ID: "7", Name: "Liam O'Brien", Email: "liam.obrien@example.com"},
	{ID: "8", Name: "Sofia Rossi", Email: "sofia.rossi@example.com"},
	{ID: "9", Name: "Kwame Asante", Email: "kwame.asante@example.com"},
	{ID: "10", Name: "Ingrid Nilsson", Email: "ingrid.nilsson@example.com"},
	{ID: "11", Name: "Arjun Sharma", Email: "arjun.sharma@example.com"},
	{ID: "12", Name: "Elena Volkov", Email: "elena.volkov@example.com"},
	{ID: "13", Name: "Mateo Silva", Email: "mateo.silva@example.com"},
	{ID: "14", Name: "Aisha Bello", Email: "aisha.bello@example.com"},
	{ID: "15", Name: "Thomas Weber", Email: "thomas.weber@example.com"},
	{ID: "16", Name: "Nadia Haddad", Email: "nadia.haddad@example.com"},
	{ID: "17", Name: "Jin-ho Park", Email: "jinho.park@example.com"},
	{ID: "18", Name: "Olivia Bennett", Email: "olivia.bennett@example.com"},
	{ID: "19", Name: "Samuel Adeyemi", Email: "samuel.adeyemi@example.com"},
	{ID: "20", Name: "Carmen Reyes", Email: "carmen.reyes@example.com"},
	{ID: "21", Name: "Henrik Larsen", Email: "henrik.larsen@example.com"},
	{ID: "22", Name: "Meera Krishnan", Email: "meera.krishnan@example.com"},
	{ID: "23", Name: "Luca Bianchi", Email: "luca.bianchi@example.com"},
	{ID: "24", Name: "Zara Ahmed", Email: "zara.ahmed@example.com"},
	{ID: "25", Name: "Gabriel Martinez", Email: "gabriel.martinez@example.com"},
}

// searchContacts returns contacts whose name or email contains query
// (case-insensitive). Empty query returns the full directory.
func searchContacts(query string) []Contact {
	if query == "" {
		return slices.Clone(contactDirectory)
	}
	q := strings.ToLower(query)
	out := make([]Contact, 0, len(contactDirectory))
	for _, c := range contactDirectory {
		if strings.Contains(strings.ToLower(c.Name), q) || strings.Contains(strings.ToLower(c.Email), q) {
			out = append(out, c)
		}
	}
	return out
}

// filterDataset is the set of items used by URL-Preserved Filters.
// Mix of active/completed statuses and dates spanning ~1 year.
var filterDataset = []FilterItem{
	{ID: "1", Name: "Design homepage", Status: "completed", Date: "2024-02-14"},
	{ID: "2", Name: "Draft Q1 proposal", Status: "completed", Date: "2024-03-01"},
	{ID: "3", Name: "Review authentication spec", Status: "active", Date: "2024-03-15"},
	{ID: "4", Name: "Migrate legacy database", Status: "active", Date: "2024-04-02"},
	{ID: "5", Name: "Write onboarding docs", Status: "completed", Date: "2024-04-20"},
	{ID: "6", Name: "Upgrade CI pipeline", Status: "active", Date: "2024-05-08"},
	{ID: "7", Name: "Host team offsite", Status: "completed", Date: "2024-06-12"},
	{ID: "8", Name: "Ship payments beta", Status: "active", Date: "2024-07-04"},
	{ID: "9", Name: "Audit access controls", Status: "completed", Date: "2024-08-19"},
	{ID: "10", Name: "Refactor billing module", Status: "active", Date: "2024-09-28"},
	{ID: "11", Name: "Launch mobile app", Status: "active", Date: "2024-11-11"},
	{ID: "12", Name: "Year-end retrospective", Status: "active", Date: "2024-12-30"},
}

// filterItems returns filterDataset filtered by status and sorted by sort key.
// Unknown status values are treated as "all"; unknown sort values fall back to "name".
func filterItems(status, sort string) []FilterItem {
	out := make([]FilterItem, 0, len(filterDataset))
	for _, item := range filterDataset {
		if status == "active" || status == "completed" {
			if item.Status != status {
				continue
			}
		}
		out = append(out, item)
	}
	switch sort {
	case "date":
		slices.SortFunc(out, func(a, b FilterItem) int {
			return strings.Compare(b.Date, a.Date) // descending (newest first)
		})
	default: // "name" and any unknown value
		slices.SortFunc(out, func(a, b FilterItem) int {
			return strings.Compare(a.Name, b.Name)
		})
	}
	return out
}

// PatternLink describes a single pattern in the index page catalog.
type PatternLink struct {
	Name        string
	Path        string
	Description string
	Implemented bool
}

// PatternCategory groups related patterns for the index page.
type PatternCategory struct {
	Name     string
	Patterns []PatternLink
}

func allPatterns() []PatternCategory {
	return []PatternCategory{
		{
			Name: "Forms & Editing",
			Patterns: []PatternLink{
				{Name: "Click To Edit", Path: "/patterns/forms/click-to-edit", Description: "Toggle between view and edit mode", Implemented: true},
				{Name: "Edit Row", Path: "/patterns/forms/edit-row", Description: "Inline editing of table rows", Implemented: true},
				{Name: "Inline Validation", Path: "/patterns/forms/inline-validation", Description: "Server-side field validation as you type", Implemented: true},
				{Name: "Bulk Update", Path: "/patterns/forms/bulk-update", Description: "Batch checkbox operations", Implemented: true},
				{Name: "Reset User Input", Path: "/patterns/forms/reset-input", Description: "Auto-clear forms after submission", Implemented: true},
				{Name: "File Upload", Path: "/patterns/forms/file-upload", Description: "Standard and chunked file uploads", Implemented: true},
				{Name: "Preserving File Inputs", Path: "/patterns/forms/preserve-inputs", Description: "Retain form values across re-renders", Implemented: true},
			},
		},
		{
			Name: "Lists & Data",
			Patterns: []PatternLink{
				{Name: "Delete Row", Path: "/patterns/lists/delete-row", Description: "Animated row removal", Implemented: true},
				{Name: "Click To Load", Path: "/patterns/lists/click-to-load", Description: "Append-only pagination", Implemented: true},
				{Name: "Infinite Scroll", Path: "/patterns/lists/infinite-scroll", Description: "Auto-load on scroll with IntersectionObserver", Implemented: true},
				{Name: "Value Select", Path: "/patterns/lists/value-select", Description: "Cascading dependent selects", Implemented: true},
			},
		},
		{
			Name: "Search & Filtering",
			Patterns: []PatternLink{
				{Name: "Active Search", Path: "/patterns/search/active-search", Description: "Debounced live search", Implemented: true},
				{Name: "URL-Preserved Filters", Path: "/patterns/search/url-filters", Description: "Bookmarkable filter state via query params", Implemented: true},
			},
		},
		{
			Name: "Loading & Progress",
			Patterns: []PatternLink{
				{Name: "Lazy Loading", Path: "/patterns/loading/lazy-loading", Description: "Load content after page render via server push"},
				{Name: "Progress Bar", Path: "/patterns/loading/progress-bar", Description: "WebSocket-pushed progress updates"},
				{Name: "Async Operations", Path: "/patterns/loading/async-operations", Description: "Loading/success/error state machine"},
			},
		},
		{
			Name: "Dialogs, Tabs & Navigation",
			Patterns: []PatternLink{
				{Name: "Modal Dialog", Path: "/patterns/navigation/modal-dialog", Description: "Native dialog with command/commandfor"},
				{Name: "Confirm Dialog", Path: "/patterns/navigation/confirm-dialog", Description: "CSP-compliant confirmation flow"},
				{Name: "Tabs (HATEOAS)", Path: "/patterns/navigation/tabs", Description: "Server-driven tabs via SPA navigation"},
				{Name: "SPA Navigation", Path: "/patterns/navigation/spa-navigation", Description: "Auto link interception with pushState"},
				{Name: "Keyboard Shortcuts", Path: "/patterns/navigation/keyboard-shortcuts", Description: "Global keyboard event binding"},
			},
		},
		{
			Name: "Visual Feedback",
			Patterns: []PatternLink{
				{Name: "Animations", Path: "/patterns/feedback/animations", Description: "Entry animations with lvt-fx:animate"},
				{Name: "Loading States", Path: "/patterns/feedback/loading-states", Description: "Auto aria-busy and custom loading text"},
				{Name: "Highlight on Change", Path: "/patterns/feedback/highlight", Description: "Visual flash on DOM updates"},
				{Name: "Flash Messages", Path: "/patterns/feedback/flash-messages", Description: "Toast notifications via ctx.SetFlash"},
			},
		},
		{
			Name: "Real-Time & Multi-User",
			Patterns: []PatternLink{
				{Name: "Multi-User Sync", Path: "/patterns/realtime/multi-user-sync", Description: "Auto-sync across tabs via Sync() handler"},
				{Name: "Broadcasting", Path: "/patterns/realtime/broadcasting", Description: "Cross-connection updates via BroadcastAction"},
				{Name: "Presence Tracking", Path: "/patterns/realtime/presence", Description: "Explicit join/leave with shared state"},
				{Name: "Reconnection Recovery", Path: "/patterns/realtime/reconnection", Description: "State persistence across disconnects"},
				{Name: "Live Preview", Path: "/patterns/realtime/live-preview", Description: "Real-time input preview via Change()"},
				{Name: "Server Push", Path: "/patterns/realtime/server-push", Description: "Background goroutine pushing updates"},
			},
		},
	}
}
