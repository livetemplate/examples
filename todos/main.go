package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
	"todos/db"
)

var validate = validator.New()

// TodoItem is an alias for the database model
type TodoItem = db.Todo

type AddInput struct {
	Text string `json:"text" validate:"required,min=3"`
}

type ToggleInput struct {
	ID string `json:"id" validate:"required"`
}

type DeleteInput struct {
	ID string `json:"id" validate:"required"`
}

type SearchInput struct {
	Query string `json:"query"`
}

type SortInput struct {
	SortBy string `json:"sort_by"`
}

type PaginationInput struct {
	Page int `json:"page" validate:"required,min=1"`
}

// TodoState demonstrates the Controller pattern with automatic method dispatch.
//
// Instead of implementing the Store interface with a Change() switch statement,
// each action maps directly to a method:
//
//   - "add"             → Add(ctx)
//   - "toggle"          → Toggle(ctx)
//   - "delete"          → Delete(ctx)
//   - "search"          → Search(ctx)
//   - "sort"            → Sort(ctx)
//   - "next_page"       → NextPage(ctx)
//   - "prev_page"       → PrevPage(ctx)
//   - "goto_page"       → GotoPage(ctx)
//   - "clear_completed" → ClearCompleted(ctx)
//
// Actions support both snake_case (next_page) and camelCase (nextPage).
type TodoState struct {
	Title          string      `json:"title"`
	Queries        *db.Queries `json:"-"` // Database queries (exported but not in JSON)
	SearchQuery    string      `json:"search_query"`
	SortBy         string      `json:"sort_by"`
	FilteredTodos  []TodoItem  `json:"filtered_todos"`
	CurrentPage    int         `json:"current_page"`
	PageSize       int         `json:"page_size"`
	TotalPages     int         `json:"total_pages"`
	PaginatedTodos []TodoItem  `json:"paginated_todos"`
	TotalCount     int         `json:"total_count"`
	CompletedCount int         `json:"completed_count"`
	RemainingCount int         `json:"remaining_count"`
	LastUpdated    string      `json:"last_updated"`
	ShowPagination bool        `json:"show_pagination"`
	PrevDisabled   bool        `json:"prev_disabled"`
	NextDisabled   bool        `json:"next_disabled"`
}

// Add handles the "add" action - creates a new todo item
func (s *TodoState) Add(ctx *livetemplate.ActionContext) error {
	var input AddInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}

	// Generate unique ID and timestamp
	now := time.Now()
	id := fmt.Sprintf("todo-%d", now.UnixNano())
	dbCtx := context.Background()

	// Insert into database
	_, err := s.Queries.CreateTodo(dbCtx, db.CreateTodoParams{
		ID:        id,
		Text:      input.Text,
		Completed: false,
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("failed to create todo: %w", err)
	}

	s.LastUpdated = formatTime()
	return s.loadTodos(dbCtx)
}

// Toggle handles the "toggle" action - toggles todo completion status
func (s *TodoState) Toggle(ctx *livetemplate.ActionContext) error {
	var input ToggleInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}

	dbCtx := context.Background()

	// Get current todo to toggle its completed status
	todo, err := s.Queries.GetTodoByID(dbCtx, input.ID)
	if err != nil {
		return fmt.Errorf("failed to get todo: %w", err)
	}

	// Update in database
	err = s.Queries.UpdateTodoCompleted(dbCtx, db.UpdateTodoCompletedParams{
		Completed: !todo.Completed,
		ID:        input.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}

	s.LastUpdated = formatTime()
	return s.loadTodos(dbCtx)
}

// Delete handles the "delete" action - removes a todo item
func (s *TodoState) Delete(ctx *livetemplate.ActionContext) error {
	var input DeleteInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}

	dbCtx := context.Background()

	// Delete from database
	err := s.Queries.DeleteTodo(dbCtx, input.ID)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	s.LastUpdated = formatTime()
	return s.loadTodos(dbCtx)
}

// Search handles the "search" action - filters todos by search query
func (s *TodoState) Search(ctx *livetemplate.ActionContext) error {
	var input SearchInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}
	s.SearchQuery = input.Query
	s.LastUpdated = formatTime()
	return s.loadTodos(context.Background())
}

// Sort handles the "sort" action - changes todo sort order
func (s *TodoState) Sort(ctx *livetemplate.ActionContext) error {
	var input SortInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}
	s.SortBy = input.SortBy
	s.LastUpdated = formatTime()
	return s.loadTodos(context.Background())
}

// NextPage handles the "next_page" action - navigates to next page
func (s *TodoState) NextPage(ctx *livetemplate.ActionContext) error {
	if s.CurrentPage < s.TotalPages {
		s.CurrentPage++
	}
	s.LastUpdated = formatTime()
	return s.loadTodos(context.Background())
}

// PrevPage handles the "prev_page" action - navigates to previous page
func (s *TodoState) PrevPage(ctx *livetemplate.ActionContext) error {
	if s.CurrentPage > 1 {
		s.CurrentPage--
	}
	s.LastUpdated = formatTime()
	return s.loadTodos(context.Background())
}

// GotoPage handles the "goto_page" action - navigates to specific page
func (s *TodoState) GotoPage(ctx *livetemplate.ActionContext) error {
	var input PaginationInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}
	if input.Page >= 1 && input.Page <= s.TotalPages {
		s.CurrentPage = input.Page
	}
	s.LastUpdated = formatTime()
	return s.loadTodos(context.Background())
}

// ClearCompleted handles the "clear_completed" action - removes all completed todos
func (s *TodoState) ClearCompleted(ctx *livetemplate.ActionContext) error {
	dbCtx := context.Background()

	// Delete all completed todos from database
	err := s.Queries.DeleteCompletedTodos(dbCtx)
	if err != nil {
		return fmt.Errorf("failed to delete completed todos: %w", err)
	}

	s.LastUpdated = formatTime()
	return s.loadTodos(dbCtx)
}

// Init implements livetemplate.StoreInitializer
// This is called when the store is cloned for a new session (e.g., page refresh)
func (s *TodoState) Init() error {
	return s.loadTodos(context.Background())
}

// loadTodos loads todos from database and updates computed fields
func (s *TodoState) loadTodos(ctx context.Context) error {
	// Get all todos from database
	todos, err := s.Queries.GetAllTodos(ctx)
	if err != nil {
		return fmt.Errorf("failed to load todos: %w", err)
	}

	// Apply search filter
	if s.SearchQuery == "" {
		s.FilteredTodos = todos
	} else {
		s.FilteredTodos = []TodoItem{}
		query := strings.ToLower(s.SearchQuery)
		for _, todo := range todos {
			if strings.Contains(strings.ToLower(todo.Text), query) {
				s.FilteredTodos = append(s.FilteredTodos, todo)
			}
		}
	}

	// Update statistics
	s.TotalCount = len(todos)
	s.CompletedCount = 0
	for _, todo := range todos {
		if todo.Completed {
			s.CompletedCount++
		}
	}
	s.RemainingCount = s.TotalCount - s.CompletedCount

	// Apply sorting and pagination
	s.applySorting()
	s.applyPagination()

	return nil
}

func (s *TodoState) updateStats() {
	// Stats are now calculated in loadTodos
	// This is kept for backward compatibility but does nothing
}

func (s *TodoState) updateFilteredTodos() {
	// Filtering is now done in loadTodos
	// This is kept for backward compatibility but does nothing
}

func (s *TodoState) applySorting() {
	switch s.SortBy {
	case "alphabetical":
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(s.FilteredTodos[i].Text) < strings.ToLower(s.FilteredTodos[j].Text)
		})
	case "reverse_alphabetical":
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(s.FilteredTodos[i].Text) > strings.ToLower(s.FilteredTodos[j].Text)
		})
	case "oldest_first":
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return s.FilteredTodos[i].CreatedAt.Before(s.FilteredTodos[j].CreatedAt)
		})
	default:
		// Default: newest first (reverse chronological)
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return s.FilteredTodos[i].CreatedAt.After(s.FilteredTodos[j].CreatedAt)
		})
	}
}

func (s *TodoState) applyPagination() {
	// Calculate total pages
	if len(s.FilteredTodos) == 0 {
		s.TotalPages = 1
		s.CurrentPage = 1
		s.PaginatedTodos = []TodoItem{}
		s.ShowPagination = false
		s.PrevDisabled = true
		s.NextDisabled = true
		return
	}

	s.TotalPages = int(math.Ceil(float64(len(s.FilteredTodos)) / float64(s.PageSize)))

	// Validate and adjust current page if needed
	if s.CurrentPage < 1 {
		s.CurrentPage = 1
	}
	if s.CurrentPage > s.TotalPages {
		s.CurrentPage = s.TotalPages
	}

	// Calculate start and end indices for current page
	start := (s.CurrentPage - 1) * s.PageSize
	end := start + s.PageSize
	if end > len(s.FilteredTodos) {
		end = len(s.FilteredTodos)
	}

	// Slice to get current page items
	s.PaginatedTodos = s.FilteredTodos[start:end]

	hasPaginated := len(s.PaginatedTodos) > 0
	s.ShowPagination = hasPaginated && s.TotalPages > 1
	s.PrevDisabled = !hasPaginated || s.CurrentPage <= 1
	s.NextDisabled = !hasPaginated || s.CurrentPage >= s.TotalPages
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("LiveTemplate Todo App starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize database
	dbPath := GetDBPath()
	queries, dbErr := InitDB(dbPath)
	if dbErr != nil {
		log.Fatalf("Failed to initialize database: %v", dbErr)
	}
	defer CloseDB()

	// Create initial state
	state := &TodoState{
		Title:       "Todo App",
		Queries:     queries,
		CurrentPage: 1,
		PageSize:    3,
		LastUpdated: formatTime(),
	}

	// Load initial todos from database
	if err := state.loadTodos(context.Background()); err != nil {
		log.Fatalf("Failed to load initial todos: %v", err)
	}

	// Create template with environment-based configuration
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.Must(livetemplate.New("todos", envConfig.ToOptions()...))

	// Mount handler - auto-handles initial page, WebSocket, and HTTP actions
	http.Handle("/", tmpl.Handle(state))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)

	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
