package main

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
	ID   string
	Name string
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
				{Name: "Delete Row", Path: "/patterns/lists/delete-row", Description: "Animated row removal"},
				{Name: "Click To Load", Path: "/patterns/lists/click-to-load", Description: "Append-only pagination"},
				{Name: "Infinite Scroll", Path: "/patterns/lists/infinite-scroll", Description: "Auto-load on scroll with IntersectionObserver"},
				{Name: "Value Select", Path: "/patterns/lists/value-select", Description: "Cascading dependent selects"},
			},
		},
		{
			Name: "Search & Filtering",
			Patterns: []PatternLink{
				{Name: "Active Search", Path: "/patterns/search/active-search", Description: "Debounced live search"},
				{Name: "URL-Preserved Filters", Path: "/patterns/search/url-filters", Description: "Bookmarkable filter state via query params"},
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
