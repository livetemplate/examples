package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/livetemplate/examples/todos/db"
	"github.com/livetemplate/livetemplate"
	"github.com/livetemplate/lvt/components/modal"
	"github.com/livetemplate/lvt/components/toast"
)

type TodoController struct {
	Queries *db.Queries
}

func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.Username = ctx.UserID()
	state = initComponents(state)
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) OnConnect(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.Username = ctx.UserID()
	state = initComponents(state)
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) Sync(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state = initComponents(state)
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input AddInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	now := time.Now()
	id := fmt.Sprintf("todo-%d", now.UnixNano())
	dbCtx := context.Background()

	_, err := c.Queries.CreateTodo(dbCtx, db.CreateTodoParams{
		ID:        id,
		UserID:    ctx.UserID(),
		Text:      input.Text,
		Completed: false,
		CreatedAt: now,
	})
	if err != nil {
		return state, fmt.Errorf("failed to create todo: %w", err)
	}

	state.Toasts.AddSuccess("Added", fmt.Sprintf("%q added", input.Text))
	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state, ctx.UserID())
}

func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input ToggleInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	dbCtx := context.Background()

	todo, err := c.Queries.GetTodoByID(dbCtx, db.GetTodoByIDParams{
		ID:     input.ID,
		UserID: ctx.UserID(),
	})
	if err != nil {
		return state, fmt.Errorf("failed to get todo: %w", err)
	}

	err = c.Queries.UpdateTodoCompleted(dbCtx, db.UpdateTodoCompletedParams{
		Completed: !todo.Completed,
		ID:        input.ID,
		UserID:    ctx.UserID(),
	})
	if err != nil {
		return state, fmt.Errorf("failed to update todo: %w", err)
	}

	if !todo.Completed {
		state.Toasts.AddInfo("Done", "Todo marked as complete")
	} else {
		state.Toasts.AddInfo("Reopened", "Todo marked as incomplete")
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state, ctx.UserID())
}

// ConfirmDelete shows the delete confirmation modal for the given todo ID.
func (c *TodoController) ConfirmDelete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.DeleteID = ctx.GetString("id")
	state.DeleteConfirm.Show()
	return state, nil
}

// ConfirmDeleteConfirm executes the deletion after the user confirms the modal.
func (c *TodoController) ConfirmDeleteConfirm(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.DeleteID == "" {
		return state, nil
	}

	dbCtx := context.Background()
	err := c.Queries.DeleteTodo(dbCtx, db.DeleteTodoParams{
		ID:     state.DeleteID,
		UserID: ctx.UserID(),
	})
	if err != nil {
		return state, fmt.Errorf("failed to delete todo: %w", err)
	}

	state.Toasts.AddSuccess("Deleted", "Todo removed")
	state.DeleteConfirm.Hide()
	state.DeleteID = ""
	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state, ctx.UserID())
}

// CancelDeleteConfirm dismisses the delete confirmation modal.
func (c *TodoController) CancelDeleteConfirm(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.DeleteConfirm.Hide()
	state.DeleteID = ""
	return state, nil
}

// DismissToastNotifications handles the dismiss action emitted by the toast component.
func (c *TodoController) DismissToastNotifications(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.Toasts.Dismiss(ctx.GetString("toast"))
	return state, nil
}

func (c *TodoController) Change(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if ctx.Has("query") {
		state.SearchQuery = ctx.GetString("query")
		state.CurrentPage = 1
		state.LastUpdated = formatTime()
		return c.loadTodos(context.Background(), state, ctx.UserID())
	}
	if ctx.Has("sort_by") {
		state.SortBy = ctx.GetString("sort_by")
		state.LastUpdated = formatTime()
		return c.loadTodos(context.Background(), state, ctx.UserID())
	}
	return state, nil
}

func (c *TodoController) Search(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input SearchInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	state.SearchQuery = input.Query
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) Sort(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input SortInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	state.SortBy = input.SortBy
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) NextPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.CurrentPage < state.TotalPages {
		state.CurrentPage++
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) PrevPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	if state.CurrentPage > 1 {
		state.CurrentPage--
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) GotoPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input PaginationInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	if input.Page >= 1 && input.Page <= state.TotalPages {
		state.CurrentPage = input.Page
	}
	state.LastUpdated = formatTime()
	return c.loadTodos(context.Background(), state, ctx.UserID())
}

func (c *TodoController) ClearCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	dbCtx := context.Background()

	err := c.Queries.DeleteCompletedTodos(dbCtx, ctx.UserID())
	if err != nil {
		return state, fmt.Errorf("failed to delete completed todos: %w", err)
	}

	state.Toasts.AddSuccess("Cleared", fmt.Sprintf("%d completed todo(s) removed", state.CompletedCount))
	state.LastUpdated = formatTime()
	return c.loadTodos(dbCtx, state, ctx.UserID())
}

func (c *TodoController) loadTodos(ctx context.Context, state TodoState, userID string) (TodoState, error) {
	todos, err := c.Queries.GetAllTodos(ctx, userID)
	if err != nil {
		return state, fmt.Errorf("failed to load todos: %w", err)
	}

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

	state.TotalCount = len(todos)
	state.CompletedCount = 0
	for _, todo := range todos {
		if todo.Completed {
			state.CompletedCount++
		}
	}
	state.RemainingCount = state.TotalCount - state.CompletedCount

	state = applySorting(state)
	state = applyPagination(state)

	return state, nil
}

// initComponents initializes non-serializable component objects.
// Called from Mount/OnConnect/Sync since components can't survive serialization.
func initComponents(state TodoState) TodoState {
	if state.Toasts == nil {
		toasts := toast.New("notifications",
			toast.WithPosition(toast.TopRight),
			toast.WithMaxVisible(3),
		)
		toasts.SetStyled(false)
		state.Toasts = toasts
	}
	if state.DeleteConfirm == nil {
		state.DeleteConfirm = modal.NewConfirm("delete_confirm",
			modal.WithConfirmTitle("Delete Todo"),
			modal.WithConfirmMessage("Are you sure you want to delete this todo?"),
			modal.WithConfirmDestructive(true),
			modal.WithConfirmText("Delete"),
			modal.WithCancelText("Cancel"),
		)
	}
	return state
}
