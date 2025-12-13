package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/livetemplate/livetemplate"
	"todos/db"
)

// TodoController implements the Controller+State pattern for the todo application.
//
// The controller is a singleton that holds dependencies (database connection).
// Action methods receive state as the first parameter and return modified state.
// This separation allows for easy testing and clear data flow.
//
// Actions support both snake_case (next_page) and camelCase (nextPage) naming
// due to LiveTemplate's automatic action name normalization.
type TodoController struct {
	// Queries provides access to database operations.
	// This is the only dependency held by the controller.
	Queries *db.Queries
}

// Mount is called when a new session is created.
// It loads the initial set of todos from the database.
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	return c.loadTodos(context.Background(), state)
}

// Add creates a new todo item with the given text.
// Returns validation error if text is less than 3 characters.
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input AddInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	// Generate unique ID using timestamp
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

// Toggle changes a todo's completion status (completed <-> incomplete).
func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input ToggleInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	dbCtx := context.Background()

	// Get current todo to determine its current status
	todo, err := c.Queries.GetTodoByID(dbCtx, input.ID)
	if err != nil {
		return state, fmt.Errorf("failed to get todo: %w", err)
	}

	// Toggle the completion status
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

// Delete removes a todo item by its ID.
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input DeleteInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	dbCtx := context.Background()

	err := c.Queries.DeleteTodo(dbCtx, input.ID)
	if err != nil {
		return state, fmt.Errorf("failed to delete todo: %w", err)
	}

	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state)
}

// Search filters todos by the given query string (case-insensitive substring match).
// An empty query returns all todos.
func (c *TodoController) Search(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input SearchInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	state.SearchQuery = input.Query
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// Sort changes the order in which todos are displayed.
// See SortInput for valid sort values.
func (c *TodoController) Sort(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input SortInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	state.SortBy = input.SortBy
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// NextPage navigates to the next page if available.
func (c *TodoController) NextPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.CurrentPage < state.TotalPages {
		state.CurrentPage++
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// PrevPage navigates to the previous page if available.
func (c *TodoController) PrevPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.CurrentPage > 1 {
		state.CurrentPage--
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state)
}

// GotoPage navigates directly to a specific page number.
// Invalid page numbers are ignored.
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

// ClearCompleted removes all todos that have been marked as completed.
func (c *TodoController) ClearCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	dbCtx := context.Background()

	err := c.Queries.DeleteCompletedTodos(dbCtx)
	if err != nil {
		return state, fmt.Errorf("failed to delete completed todos: %w", err)
	}

	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state)
}

// loadTodos fetches all todos from the database and updates computed fields.
// It applies search filtering, sorting, pagination, and updates statistics.
func (c *TodoController) loadTodos(ctx context.Context, state TodoState) (TodoState, error) {
	// Fetch all todos from database
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

	// Update statistics (based on all todos, not filtered)
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
