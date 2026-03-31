# LiveTemplate Todo App

A real-time todo application demonstrating LiveTemplate's controller pattern with SQLite persistence, basic authentication, search, sorting, and pagination. Styled with [Pico CSS](https://picocss.com/).

## Features

- **Basic authentication** - Per-user todo lists (alice/password, bob/password)
- **Add todos** - Create new tasks via form submission with validation
- **Toggle completion** - Mark tasks as done/undone with checkboxes
- **Delete todos** - Remove individual tasks
- **Clear completed** - Bulk remove all completed tasks
- **Search** - Filter todos by text
- **Sort** - Newest first, oldest first, alphabetical (A-Z / Z-A)
- **Pagination** - 3 items per page with navigation controls
- **Live statistics** - Real-time total, completed, and remaining counts
- **Reactive updates** - Changes broadcast to all connected clients
- **SQLite persistence** - Todos survive server restarts

## Quick Start

```bash
cd todos
go run .
```

Open <http://localhost:8080> and log in with `alice` / `password`.

With a custom port:

```bash
PORT=8081 go run .
```

## How It Works

### Controller Pattern

The app uses LiveTemplate's controller pattern where each action maps to a typed method:

```go
type TodoController struct {
    Queries *db.Queries
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    var input AddInput
    if err := ctx.BindAndValidate(&input, validate); err != nil {
        return state, err
    }
    // Create todo in database, reload list
    return c.loadTodos(dbCtx, state, ctx.UserID())
}

func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
func (c *TodoController) ClearCompleted(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
func (c *TodoController) Search(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
func (c *TodoController) Sort(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
func (c *TodoController) NextPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
func (c *TodoController) PrevPage(state TodoState, ctx *livetemplate.Context) (TodoState, error) { ... }
```

Actions are routed from HTML via form `name` and button `name` attributes (Tier 1 pattern):

```html
<!-- Form name="add" routes to Add() method -->
<form method="POST" name="add">
    <input type="text" name="text" placeholder="What needs to be done?" required />
    <button type="submit" name="add">Add</button>
</form>

<!-- Hidden input passes data; form name routes to Toggle() -->
<form method="POST" name="toggle">
    <input type="hidden" name="id" value="{{ .ID }}" />
    <input type="checkbox" onchange="this.form.requestSubmit()" />
</form>

<!-- Button name routes to ClearCompleted() -->
<button name="clearCompleted">Clear Completed</button>
```

### Authentication

Basic auth with hardcoded demo users. `ctx.UserID()` returns the authenticated username, used to isolate each user's todos in SQLite:

```go
auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
    users := map[string]string{"alice": "password", "bob": "password"}
    pass, ok := users[username]
    return ok && pass == password, nil
})
```

### Database

SQLite via [sqlc](https://sqlc.dev/)-generated queries. The `db/` directory contains generated code from `queries.sql`. Schema migrations run automatically on startup, including detection and recreation of outdated schemas.

## Testing

### Browser E2E Test

```bash
go test -v -run TestTodosE2E
```

Requires Docker for Chrome headless testing.

## Development Notes

- **Port**: Defaults to `:8080`, override with `PORT` environment variable
- **Database**: `todos.db` in the current directory (`:memory:` when `TEST_MODE=1`)
- **Client Library**: Served via `e2etest.ServeClientLibrary` in dev mode, CDN in production
