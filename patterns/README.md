# LiveTemplate Patterns

A catalog of 31 reactive UI patterns implemented with [LiveTemplate](https://github.com/livetemplate/livetemplate). Each pattern is a self-contained handler demonstrating a single idiom — forms, lists, navigation, real-time, and more.

The patterns trace the [htmx examples](https://htmx.org/examples/) and [Phoenix LiveView patterns](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html), translated to LiveTemplate's controller + state model. Styled with [Pico CSS](https://picocss.com/).

## Quick Start

```bash
cd examples
GOWORK=off go run ./patterns
```

Open <http://localhost:8080> for the live index — every pattern links to its own page.

```bash
PORT=8081 GOWORK=off go run ./patterns
```

## What's Here

### Forms & Editing

- **Click To Edit** — toggle between view and edit mode
- **Edit Row** — inline editing of table rows
- **Inline Validation** — server-side field validation as you type
- **Bulk Update** — batch checkbox operations
- **Reset User Input** — auto-clear forms after submission
- **File Upload** — standard and chunked file uploads
- **Preserving File Inputs** — retain form values across re-renders

### Lists & Data

- **Delete Row** — animated row removal
- **Click To Load** — append-only pagination
- **Infinite Scroll** — auto-load on scroll with `lvt-scroll-sentinel`
- **Value Select** — cascading dependent selects

### Search & Filtering

- **Active Search** — debounced live search
- **URL-Preserved Filters** — bookmarkable filter state via query params

### Loading & Progress

- **Lazy Loading** — load content after page render via server push
- **Progress Bar** — WebSocket-pushed progress updates
- **Async Operations** — loading/success/error state machine

### Dialogs, Tabs & Navigation

- **Modal Dialog** — native dialog with `command`/`commandfor`
- **Confirm Dialog** — CSP-compliant confirmation flow
- **Tabs (HATEOAS)** — server-driven tabs via SPA navigation
- **SPA Navigation** — auto link interception with pushState
- **Keyboard Shortcuts** — global keyboard event binding

### Visual Feedback

- **Animations** — entry animations with `lvt-fx:animate`
- **Loading States** — auto `aria-busy` and custom loading text
- **Highlight on Change** — visual flash on DOM updates
- **Flash Messages** — toast notifications via `ctx.SetFlash`

### Real-Time & Multi-User

- **Multi-User Sync** — auto-sync across tabs via `Sync()` handler
- **Broadcasting** — cross-connection updates via `BroadcastAction`
- **Presence Tracking** — explicit join/leave with shared state
- **Reconnection Recovery** — state persistence via `lvt:"persist"`
- **Live Preview** — real-time input preview via `Change()`
- **Server Push** — background goroutine pushing updates with `session.TriggerAction`

## Architecture

One handler per pattern, grouped by category:

```
patterns/
├── main.go                       # Mux wiring + index handler
├── data.go                       # Sample data + pattern catalog (drives the index page)
├── state_{category}.go           # State structs per category (Forms, Lists, …)
├── handlers_{category}.go        # Controllers + action methods per category
├── templates/
│   ├── layout.tmpl               # Shared shell (Pico CSS, breadcrumb)
│   ├── index.tmpl                # Catalog page rendered from data.go
│   └── {category}/*.tmpl         # One template per pattern
└── patterns_test.go              # E2E tests (chromedp)
```

Each pattern follows the [controller + state convention](https://github.com/livetemplate/livetemplate/blob/main/docs/references/controller-pattern.md): controllers are singletons holding dependencies; state is a plain struct cloned per session. Action methods have signature `func (c *Controller) Action(state State, ctx *Context) (State, error)`.

The index page is data-driven by `data.go :: allPatterns()` — adding or renaming a pattern is a single struct edit.

## Testing

```bash
# Full E2E suite (requires Docker for the chromedp container)
unset PORT && GOWORK=off go test -v -race -timeout=10m ./patterns

# Visual checks (LLM-validated screenshots, opt-in)
unset PORT && LVT_VISUAL_CHECK=true GOWORK=off go test -v ./patterns -run "Visual_Check"

# Single pattern
GOWORK=off go test -v ./patterns -run TestEditRow
```

Multi-tab tests (`TestMultiUserSync`, `TestBroadcasting`, `TestPresence`) use `chromedp.NewContext(parentCtx)` to open a second tab on the same allocator, sharing the session-group cookie. See `setupPeerTab` in `patterns_test.go`.

E2E tests have access to browser console logs, server logs, WebSocket messages, and rendered HTML — see [`livetemplate/CLAUDE.md`](https://github.com/livetemplate/livetemplate/blob/main/CLAUDE.md) for the full E2E contract.

## Reference

- [`docs/proposals/patterns.md`](https://github.com/livetemplate/livetemplate/blob/main/docs/proposals/patterns.md) — original proposal driving the implementation
- [`examples/CLAUDE.md`](../CLAUDE.md) — examples-repo conventions (Tier 1 vs Tier 2, Pico/CSP boilerplate)
- [`livetemplate/CLAUDE.md`](https://github.com/livetemplate/livetemplate/blob/main/CLAUDE.md) — controller pattern, `data-key`, AI review loop
- [Progressive Complexity Reference](https://github.com/livetemplate/livetemplate/blob/main/docs/references/progressive-complexity-reference.md) — Tier 1 vs Tier 2
- [Client Attributes Reference](https://github.com/livetemplate/livetemplate/blob/main/docs/references/client-attributes.md) — `lvt-*` attribute listing
