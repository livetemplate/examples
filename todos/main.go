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

// TodoPrefs holds user preferences that persist across sessions.
// These are the minimal settings needed to restore the user's view state.
type TodoPrefs struct {
	SearchQuery string `json:"search_query"`
	SortBy      string `json:"sort_by"`
	CurrentPage int    `json:"current_page"`
	PageSize    int    `json:"page_size"`
}

// TodoView holds computed/derived fields for template rendering.
// These are recalculated by loadTodos() and NOT persisted to Redis.
type TodoView struct {
	FilteredTodos  []TodoItem `json:"filtered_todos"`
	PaginatedTodos []TodoItem `json:"paginated_todos"`
	TotalCount     int        `json:"total_count"`
	CompletedCount int        `json:"completed_count"`
	RemainingCount int        `json:"remaining_count"`
	TotalPages     int        `json:"total_pages"`
	ShowPagination bool       `json:"show_pagination"`
	PrevDisabled   bool       `json:"prev_disabled"`
	NextDisabled   bool       `json:"next_disabled"`
	LastUpdated    string     `json:"last_updated"`
}

// TodoController demonstrates the Controller pattern with automatic method dispatch
// and proper state separation using the lvt:state tag.
//
// Fields tagged with `lvt:"state"` are persisted to Redis/session storage.
// Fields without the tag (dependencies like DB) come from the template clone.
//
// Note: The DB field must be exported for reflection-based cloning to work.
// Use `json:"-"` to exclude it from JSON serialization.
//
// Actions are automatically dispatched to methods:
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
type TodoController struct {
	// Configuration (static, copied from template)
	Title string `json:"title"`

	// State fields - PERSISTED to Redis via lvt:"state" tag
	Prefs *TodoPrefs `json:"prefs" lvt:"state"`

	// Computed fields - NOT persisted, recalculated on Init()
	View *TodoView `json:"view"`

	// Dependencies - NOT persisted, come from template clone
	// Must be exported for reflection-based cloning; json:"-" excludes from serialization
	DB *db.Queries `json:"-"`
}

// Add handles the "add" action - creates a new todo item
func (c *TodoController) Add(ctx *livetemplate.ActionContext) error {
	var input AddInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}

	// Generate unique ID and timestamp
	now := time.Now()
	id := fmt.Sprintf("todo-%d", now.UnixNano())
	dbCtx := context.Background()

	// Insert into database
	_, err := c.DB.CreateTodo(dbCtx, db.CreateTodoParams{
		ID:        id,
		Text:      input.Text,
		Completed: false,
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("failed to create todo: %w", err)
	}

	c.View.LastUpdated = formatTime()
	return c.loadTodos(dbCtx)
}

// Toggle handles the "toggle" action - toggles todo completion status
func (c *TodoController) Toggle(ctx *livetemplate.ActionContext) error {
	var input ToggleInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}

	dbCtx := context.Background()

	// Get current todo to toggle its completed status
	todo, err := c.DB.GetTodoByID(dbCtx, input.ID)
	if err != nil {
		return fmt.Errorf("failed to get todo: %w", err)
	}

	// Update in database
	err = c.DB.UpdateTodoCompleted(dbCtx, db.UpdateTodoCompletedParams{
		Completed: !todo.Completed,
		ID:        input.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}

	c.View.LastUpdated = formatTime()
	return c.loadTodos(dbCtx)
}

// Delete handles the "delete" action - removes a todo item
func (c *TodoController) Delete(ctx *livetemplate.ActionContext) error {
	var input DeleteInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}

	dbCtx := context.Background()

	// Delete from database
	err := c.DB.DeleteTodo(dbCtx, input.ID)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	c.View.LastUpdated = formatTime()
	return c.loadTodos(dbCtx)
}

// Search handles the "search" action - filters todos by search query
func (c *TodoController) Search(ctx *livetemplate.ActionContext) error {
	var input SearchInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}
	c.Prefs.SearchQuery = input.Query
	c.View.LastUpdated = formatTime()
	return c.loadTodos(context.Background())
}

// Sort handles the "sort" action - changes todo sort order
func (c *TodoController) Sort(ctx *livetemplate.ActionContext) error {
	var input SortInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}
	c.Prefs.SortBy = input.SortBy
	c.View.LastUpdated = formatTime()
	return c.loadTodos(context.Background())
}

// NextPage handles the "next_page" action - navigates to next page
func (c *TodoController) NextPage(ctx *livetemplate.ActionContext) error {
	if c.Prefs.CurrentPage < c.View.TotalPages {
		c.Prefs.CurrentPage++
	}
	c.View.LastUpdated = formatTime()
	return c.loadTodos(context.Background())
}

// PrevPage handles the "prev_page" action - navigates to previous page
func (c *TodoController) PrevPage(ctx *livetemplate.ActionContext) error {
	if c.Prefs.CurrentPage > 1 {
		c.Prefs.CurrentPage--
	}
	c.View.LastUpdated = formatTime()
	return c.loadTodos(context.Background())
}

// GotoPage handles the "goto_page" action - navigates to specific page
func (c *TodoController) GotoPage(ctx *livetemplate.ActionContext) error {
	var input PaginationInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return err
	}
	if input.Page >= 1 && input.Page <= c.View.TotalPages {
		c.Prefs.CurrentPage = input.Page
	}
	c.View.LastUpdated = formatTime()
	return c.loadTodos(context.Background())
}

// ClearCompleted handles the "clear_completed" action - removes all completed todos
func (c *TodoController) ClearCompleted(ctx *livetemplate.ActionContext) error {
	dbCtx := context.Background()

	// Delete all completed todos from database
	err := c.DB.DeleteCompletedTodos(dbCtx)
	if err != nil {
		return fmt.Errorf("failed to delete completed todos: %w", err)
	}

	c.View.LastUpdated = formatTime()
	return c.loadTodos(dbCtx)
}

// Init implements livetemplate.StoreInitializer
// This is called when the store is cloned for a new session (e.g., page refresh)
// For session restore: Template clone provides db, lvt:state injects Prefs, then Init() recalculates View
func (c *TodoController) Init() error {
	// Ensure Prefs exists (may be nil for new sessions)
	if c.Prefs == nil {
		c.Prefs = &TodoPrefs{
			CurrentPage: 1,
			PageSize:    3,
		}
	}

	// Ensure View exists
	if c.View == nil {
		c.View = &TodoView{}
	}

	// Load and compute view from database
	return c.loadTodos(context.Background())
}

// loadTodos loads todos from database and updates computed fields in View
func (c *TodoController) loadTodos(ctx context.Context) error {
	// Get all todos from database
	todos, err := c.DB.GetAllTodos(ctx)
	if err != nil {
		return fmt.Errorf("failed to load todos: %w", err)
	}

	// Apply search filter
	if c.Prefs.SearchQuery == "" {
		c.View.FilteredTodos = todos
	} else {
		c.View.FilteredTodos = []TodoItem{}
		query := strings.ToLower(c.Prefs.SearchQuery)
		for _, todo := range todos {
			if strings.Contains(strings.ToLower(todo.Text), query) {
				c.View.FilteredTodos = append(c.View.FilteredTodos, todo)
			}
		}
	}

	// Update statistics
	c.View.TotalCount = len(todos)
	c.View.CompletedCount = 0
	for _, todo := range todos {
		if todo.Completed {
			c.View.CompletedCount++
		}
	}
	c.View.RemainingCount = c.View.TotalCount - c.View.CompletedCount

	// Apply sorting and pagination
	c.applySorting()
	c.applyPagination()

	return nil
}

func (c *TodoController) applySorting() {
	switch c.Prefs.SortBy {
	case "alphabetical":
		sort.Slice(c.View.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(c.View.FilteredTodos[i].Text) < strings.ToLower(c.View.FilteredTodos[j].Text)
		})
	case "reverse_alphabetical":
		sort.Slice(c.View.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(c.View.FilteredTodos[i].Text) > strings.ToLower(c.View.FilteredTodos[j].Text)
		})
	case "oldest_first":
		sort.Slice(c.View.FilteredTodos, func(i, j int) bool {
			return c.View.FilteredTodos[i].CreatedAt.Before(c.View.FilteredTodos[j].CreatedAt)
		})
	default:
		// Default: newest first (reverse chronological)
		sort.Slice(c.View.FilteredTodos, func(i, j int) bool {
			return c.View.FilteredTodos[i].CreatedAt.After(c.View.FilteredTodos[j].CreatedAt)
		})
	}
}

func (c *TodoController) applyPagination() {
	// Calculate total pages
	if len(c.View.FilteredTodos) == 0 {
		c.View.TotalPages = 1
		c.Prefs.CurrentPage = 1
		c.View.PaginatedTodos = []TodoItem{}
		c.View.ShowPagination = false
		c.View.PrevDisabled = true
		c.View.NextDisabled = true
		return
	}

	c.View.TotalPages = int(math.Ceil(float64(len(c.View.FilteredTodos)) / float64(c.Prefs.PageSize)))

	// Validate and adjust current page if needed
	if c.Prefs.CurrentPage < 1 {
		c.Prefs.CurrentPage = 1
	}
	if c.Prefs.CurrentPage > c.View.TotalPages {
		c.Prefs.CurrentPage = c.View.TotalPages
	}

	// Calculate start and end indices for current page
	start := (c.Prefs.CurrentPage - 1) * c.Prefs.PageSize
	end := start + c.Prefs.PageSize
	if end > len(c.View.FilteredTodos) {
		end = len(c.View.FilteredTodos)
	}

	// Slice to get current page items
	c.View.PaginatedTodos = c.View.FilteredTodos[start:end]

	hasPaginated := len(c.View.PaginatedTodos) > 0
	c.View.ShowPagination = hasPaginated && c.View.TotalPages > 1
	c.View.PrevDisabled = !hasPaginated || c.Prefs.CurrentPage <= 1
	c.View.NextDisabled = !hasPaginated || c.Prefs.CurrentPage >= c.View.TotalPages
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

	// Create initial controller with dependencies and state
	controller := &TodoController{
		Title: "Todo App",
		DB:    queries, // Dependency - not persisted (json:"-")
		Prefs: &TodoPrefs{
			CurrentPage: 1,
			PageSize:    3,
		},
		View: &TodoView{
			LastUpdated: formatTime(),
		},
	}

	// Load initial todos from database
	if err := controller.loadTodos(context.Background()); err != nil {
		log.Fatalf("Failed to load initial todos: %v", err)
	}

	// Create template with environment-based configuration
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.Must(livetemplate.New("todos", envConfig.ToOptions()...))

	// Mount handler - auto-handles initial page, WebSocket, and HTTP actions
	http.Handle("/", tmpl.Handle(controller))

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
