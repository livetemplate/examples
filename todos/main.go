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

// TodoController demonstrates the Controller+State pattern.
//
// The Controller holds dependencies (database connection) and is a singleton.
// Action methods receive state as first parameter and return modified state.
//
// Actions support both snake_case (next_page) and camelCase (nextPage).
type TodoController struct {
	Queries *db.Queries // Database queries (dependency)
}

// TodoState is pure data, cloned per session.
// Contains only serializable fields - no database connections or dependencies.
type TodoState struct {
	Title          string     `json:"title"`
	SearchQuery    string     `json:"search_query"`
	SortBy         string     `json:"sort_by"`
	FilteredTodos  []TodoItem `json:"filtered_todos"`
	CurrentPage    int        `json:"current_page"`
	PageSize       int        `json:"page_size"`
	TotalPages     int        `json:"total_pages"`
	PaginatedTodos []TodoItem `json:"paginated_todos"`
	TotalCount     int        `json:"total_count"`
	CompletedCount int        `json:"completed_count"`
	RemainingCount int        `json:"remaining_count"`
	LastUpdated    string     `json:"last_updated"`
	ShowPagination bool       `json:"show_pagination"`
	PrevDisabled   bool       `json:"prev_disabled"`
	NextDisabled   bool       `json:"next_disabled"`
}

// Mount is called when a new session is created - loads initial todos
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	return c.loadTodos(context.Background(), state)
}

// Add handles the "add" action - creates a new todo item
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input AddInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	// Generate unique ID and timestamp
	now := time.Now()
	id := fmt.Sprintf("todo-%d", now.UnixNano())
	dbCtx := context.Background()

	// Insert into database
	_, err := c.Queries.CreateTodo(dbCtx, db.CreateTodoParams{
		ID:        id,
		Text:      input.Text,
		Completed: false,
		CreatedAt: now,
	})
	if err != nil {
		return state, fmt.Errorf("failed to create todo: %w", err)
	}

	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state)
}

// Toggle handles the "toggle" action - toggles todo completion status
func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input ToggleInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	dbCtx := context.Background()

	// Get current todo to toggle its completed status
	todo, err := c.Queries.GetTodoByID(dbCtx, input.ID)
	if err != nil {
		return state, fmt.Errorf("failed to get todo: %w", err)
	}

	// Update in database
	err = c.Queries.UpdateTodoCompleted(dbCtx, db.UpdateTodoCompletedParams{
		Completed: !todo.Completed,
		ID:        input.ID,
	})
	if err != nil {
		return state, fmt.Errorf("failed to update todo: %w", err)
	}

	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state)
}

// Delete handles the "delete" action - removes a todo item
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input DeleteInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	dbCtx := context.Background()

	// Delete from database
	err := c.Queries.DeleteTodo(dbCtx, input.ID)
	if err != nil {
		return state, fmt.Errorf("failed to delete todo: %w", err)
	}

	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state)
}

// Search handles the "search" action - filters todos by search query
func (c *TodoController) Search(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input SearchInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}
	state.SearchQuery = input.Query
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// Sort handles the "sort" action - changes todo sort order
func (c *TodoController) Sort(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input SortInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}
	state.SortBy = input.SortBy
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// NextPage handles the "next_page" action - navigates to next page
func (c *TodoController) NextPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.CurrentPage < state.TotalPages {
		state.CurrentPage++
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// PrevPage handles the "prev_page" action - navigates to previous page
func (c *TodoController) PrevPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.CurrentPage > 1 {
		state.CurrentPage--
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// GotoPage handles the "goto_page" action - navigates to specific page
func (c *TodoController) GotoPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input PaginationInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}
	if input.Page >= 1 && input.Page <= state.TotalPages {
		state.CurrentPage = input.Page
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// ClearCompleted handles the "clear_completed" action - removes all completed todos
func (c *TodoController) ClearCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	dbCtx := context.Background()

	// Delete all completed todos from database
	err := c.Queries.DeleteCompletedTodos(dbCtx)
	if err != nil {
		return state, fmt.Errorf("failed to delete completed todos: %w", err)
	}

	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state)
}

// loadTodos loads todos from database and updates computed fields
func (c *TodoController) loadTodos(ctx context.Context, state TodoState) (TodoState, error) {
	// Get all todos from database
	todos, err := c.Queries.GetAllTodos(ctx)
	if err != nil {
		return state, fmt.Errorf("failed to load todos: %w", err)
	}

	// Apply search filter
	if state.SearchQuery == "" {
		state.FilteredTodos = todos
	} else {
		state.FilteredTodos = []TodoItem{}
		query := strings.ToLower(state.SearchQuery)
		for _, todo := range todos {
			if strings.Contains(strings.ToLower(todo.Text), query) {
				state.FilteredTodos = append(state.FilteredTodos, todo)
			}
		}
	}

	// Update statistics
	state.TotalCount = len(todos)
	state.CompletedCount = 0
	for _, todo := range todos {
		if todo.Completed {
			state.CompletedCount++
		}
	}
	state.RemainingCount = state.TotalCount - state.CompletedCount

	// Apply sorting and pagination
	state = applySorting(state)
	state = applyPagination(state)

	return state, nil
}

func applySorting(state TodoState) TodoState {
	switch state.SortBy {
	case "alphabetical":
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(state.FilteredTodos[i].Text) < strings.ToLower(state.FilteredTodos[j].Text)
		})
	case "reverse_alphabetical":
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(state.FilteredTodos[i].Text) > strings.ToLower(state.FilteredTodos[j].Text)
		})
	case "oldest_first":
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return state.FilteredTodos[i].CreatedAt.Before(state.FilteredTodos[j].CreatedAt)
		})
	default:
		// Default: newest first (reverse chronological)
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return state.FilteredTodos[i].CreatedAt.After(state.FilteredTodos[j].CreatedAt)
		})
	}
	return state
}

func applyPagination(state TodoState) TodoState {
	// Calculate total pages
	if len(state.FilteredTodos) == 0 {
		state.TotalPages = 1
		state.CurrentPage = 1
		state.PaginatedTodos = []TodoItem{}
		state.ShowPagination = false
		state.PrevDisabled = true
		state.NextDisabled = true
		return state
	}

	state.TotalPages = int(math.Ceil(float64(len(state.FilteredTodos)) / float64(state.PageSize)))

	// Validate and adjust current page if needed
	if state.CurrentPage < 1 {
		state.CurrentPage = 1
	}
	if state.CurrentPage > state.TotalPages {
		state.CurrentPage = state.TotalPages
	}

	// Calculate start and end indices for current page
	start := (state.CurrentPage - 1) * state.PageSize
	end := start + state.PageSize
	if end > len(state.FilteredTodos) {
		end = len(state.FilteredTodos)
	}

	// Slice to get current page items
	state.PaginatedTodos = state.FilteredTodos[start:end]

	hasPaginated := len(state.PaginatedTodos) > 0
	state.ShowPagination = hasPaginated && state.TotalPages > 1
	state.PrevDisabled = !hasPaginated || state.CurrentPage <= 1
	state.NextDisabled = !hasPaginated || state.CurrentPage >= state.TotalPages
	return state
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

	// Create controller (singleton, holds dependencies)
	controller := &TodoController{
		Queries: queries,
	}

	// Create initial state (pure data, cloned per session)
	// Note: todos are loaded via Mount() when each session is created
	initialState := &TodoState{
		Title:       "Todo App",
		CurrentPage: 1,
		PageSize:    3,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.Must(livetemplate.New("todos", envConfig.ToOptions()...))

	// Mount handler with Controller+State pattern
	// - Controller: singleton with database dependency
	// - State: wrapped with AsState() for per-session cloning
	// - Mount() is called when each session is created to load initial todos
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

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
