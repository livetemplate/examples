package main

import (
	"github.com/livetemplate/examples/todos/db"
	"github.com/livetemplate/lvt/components/modal"
	"github.com/livetemplate/lvt/components/toast"
)

// Default configuration constants for the todo application.
const (
	// DefaultPageSize is the number of todos displayed per page in pagination.
	DefaultPageSize = 3

	// DefaultPage is the starting page number for pagination (1-indexed).
	DefaultPage = 1
)

// TodoItem is an alias for the database model, providing a cleaner API
// for the controller and templates.
type TodoItem = db.Todo

// AddInput represents the payload for adding a new todo item.
// Text must be at least 3 characters to ensure meaningful entries.
type AddInput struct {
	Text string `json:"text" validate:"required,min=3"`
}

// ToggleInput represents the payload for toggling a todo's completion status.
type ToggleInput struct {
	ID string `json:"id" validate:"required"`
}

// DeleteInput represents the payload for deleting a todo item.
type DeleteInput struct {
	ID string `json:"id" validate:"required"`
}

// SearchInput represents the payload for filtering todos by text search.
// An empty query returns all todos (no filtering applied).
type SearchInput struct {
	Query string `json:"query"`
}

// SortInput represents the payload for changing todo sort order.
// Valid values:
//   - "" (empty): newest first (default, reverse chronological)
//   - "alphabetical": A-Z by todo text (case-insensitive)
//   - "reverse_alphabetical": Z-A by todo text (case-insensitive)
//   - "oldest_first": chronological by created_at
type SortInput struct {
	SortBy string `json:"sort_by"`
}

// PaginationInput represents the payload for navigating to a specific page.
// Page numbers are 1-indexed.
type PaginationInput struct {
	Page int `json:"page" validate:"required,min=1"`
}

// TodoState holds all session-specific data for the todo application.
// This is pure data with no dependencies - it gets cloned per session.
// The state is serializable and can be safely passed to templates.
type TodoState struct {
	// Display metadata
	Title       string `json:"title" lvt:"persist"`
	Username    string `json:"username" lvt:"persist"`
	LastUpdated string `json:"last_updated"`

	// Filter and sort settings
	SearchQuery string `json:"search_query" lvt:"persist"`
	SortBy      string `json:"sort_by" lvt:"persist"`

	// Todo data
	FilteredTodos  []TodoItem `json:"filtered_todos"`  // After search filter applied
	PaginatedTodos []TodoItem `json:"paginated_todos"` // Current page slice

	// Statistics
	TotalCount     int `json:"total_count"`
	CompletedCount int `json:"completed_count"`
	RemainingCount int `json:"remaining_count"`

	// Pagination state
	CurrentPage    int  `json:"current_page" lvt:"persist"`
	PageSize       int  `json:"page_size" lvt:"persist"`
	TotalPages     int  `json:"total_pages"`
	ShowPagination bool `json:"show_pagination"`
	PrevDisabled   bool `json:"prev_disabled"`
	NextDisabled   bool `json:"next_disabled"`

	// Component state (non-persistent, re-initialized in Mount)
	Toasts        *toast.Container
	DeleteConfirm *modal.ConfirmModal
	DeleteID      string `json:"delete_id" lvt:"persist"`
}
