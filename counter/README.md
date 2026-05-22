# LiveTemplate Counter Example

A real-time counter application demonstrating LiveTemplate's reactive state management and tree-based optimization.

## Features

- **Reactive state**: Changes to state automatically generate updates; peer fan-out via opt-in `ctx.Subscribe(ctx.SelfTopic())` + `ctx.Publish(ctx.SelfTopic(), "Action", nil)`
- **Transport-agnostic**: Works over WebSocket or plain HTTP/AJAX
- **Minimal bandwidth**: Only the changed values are transmitted, not the entire HTML
- **No custom JavaScript**: Uses only the LiveTemplate client library
- **Template-based**: HTML is generated from Go templates with conditional rendering
- **Simple API**: Create handlers with a single method call

## Running the Example

1. **Start the server:**

   From project root:
   ```bash
   go run examples/counter/main.go
   ```

   Or from the counter directory:
   ```bash
   cd examples/counter
   go run main.go
   ```

   With custom port:
   ```bash
   PORT=8081 go run main.go
   ```

   With environment-based configuration:
   ```bash
   # Development mode with connection limits
   LVT_DEV_MODE=true LVT_MAX_CONNECTIONS=100 go run main.go

   # Production mode with allowed origins
   LVT_ALLOWED_ORIGINS="https://example.com" LVT_LOG_LEVEL=info go run main.go
   ```

2. **Open your browser:**
   Navigate to `http://localhost:8080`

3. **Interact with the counter:**
   - Click **+1** to increment the counter
   - Click **-1** to decrement the counter
   - Click **Reset** to reset to zero
   - Watch the conditional text change based on the counter value

## Configuration

This example uses LiveTemplate's environment-based configuration system. All configuration is loaded from environment variables with the `LVT_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `LVT_DEV_MODE` | `false` | Enable development mode (uses local client library) |
| `LVT_MAX_CONNECTIONS` | `0` (unlimited) | Maximum concurrent WebSocket connections |
| `LVT_MAX_CONNECTIONS_PER_GROUP` | `0` (unlimited) | Maximum connections per session group |
| `LVT_ALLOWED_ORIGINS` | empty | Comma-separated list of allowed WebSocket origins |
| `LVT_LOG_LEVEL` | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| `LVT_METRICS_ENABLED` | `true` | Enable Prometheus metrics export |
| `LVT_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |

**Example configurations:**

```bash
# Development
LVT_DEV_MODE=true LVT_LOG_LEVEL=debug go run main.go

# Production with limits
LVT_MAX_CONNECTIONS=10000 LVT_ALLOWED_ORIGINS="https://example.com" go run main.go

# Disable metrics for testing
LVT_METRICS_ENABLED=false go run main.go
```

For more details, see [CONFIGURATION.md](../../docs/CONFIGURATION.md).

## How It Works

### Server Side (Go)

The server is extremely simple with the new reactive API:

```go
// Controller: singleton, holds dependencies (none in this simple example)
type CounterController struct{}

// State: pure data, cloned per session
type CounterState struct {
    Title       string `json:"title" lvt:"persist"`
    Counter     int    `json:"counter" lvt:"persist"`
    LastUpdated string `json:"last_updated" lvt:"persist"`
}

// Named action methods — routed via <button name="increment">
func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Counter++
    state.LastUpdated = formatTime()
    return state, nil
}

func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Counter--
    state.LastUpdated = formatTime()
    return state, nil
}

func (c *CounterController) Reset(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Counter = 0
    state.LastUpdated = formatTime()
    return state, nil
}

func main() {
    envConfig, _ := livetemplate.LoadEnvConfig()

    controller := &CounterController{}
    initialState := &CounterState{Title: "Live Counter", Counter: 0, LastUpdated: formatTime()}

    tmpl := livetemplate.Must(livetemplate.New("counter", envConfig.ToOptions()...))
    http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
    http.ListenAndServe(":8080", nil)
}
```

**Key concepts:**
- **Controller+State pattern**: Controller holds dependencies, State is pure data cloned per session
- **Named action methods**: `<button name="increment">` routes to `Increment()` method
- **Auto-discovery**: Automatically finds and parses `.tmpl`, `.html`, `.gotmpl` files
- **Auto Updates**: Handle() automatically generates and sends updates after action methods return
- **Auto Cloning**: Each WebSocket connection gets its own cloned state via `AsState()`
- **Session Management**: HTTP connections automatically get session-based state persistence

### Client Side (JavaScript)

**Zero-config integration** - just add one script tag:

```html
<!-- In your template — standard HTML, no special attributes needed -->
<button name="increment">+1</button>
<button name="decrement">-1</button>
<button name="reset">Reset</button>

<!-- Auto-initializing client library -->
<script src="livetemplate-client.js"></script>
```

That's it! No JavaScript code needed. The client library auto-initializes and handles:
- **Button name routing**: `<button name="increment">` routes to `Increment()` method
- **Automatic WebSocket connection** to `/live` endpoint
- **Automatic reconnection** on disconnect (configurable)
- **Automatic DOM updates** when updates arrive
- **Event delegation** - works with dynamically updated elements

#### Sending Actions with Data

Actions use standard HTML forms with button `name` routing and hidden inputs for data:

```html
<!-- Simple action via button name -->
<button name="increment">+1</button>

<!-- Form with multiple fields -->
<form method="POST" name="add">
    <input name="title" type="text">
    <input name="priority" type="number">
    <button type="submit">Add</button>
</form>

<!-- Data via hidden inputs -->
<form method="POST">
    <input type="hidden" name="id" value="123">
    <button name="delete">Delete Item</button>
</form>
```

Form field values are accessed in the controller via `ctx.GetString()`, `ctx.GetInt()`, or `ctx.BindAndValidate()`:
```go
func (c *Controller) Delete(state State, ctx *livetemplate.Context) (State, error) {
    id := ctx.GetInt("id")
    // ...
}
```

#### Tier 2 attributes (use only when standard HTML can't express it):
- `lvt-on:click` - Route click events on non-button elements (e.g., table rows)
- `lvt-on:keydown` - Handle keyboard events
- `lvt-mod:debounce` - Custom timing control for event routing
- `lvt-fx:scroll` - Auto-scroll behavior
- `lvt-fx:animate` - Entry/exit animations
- `lvt-form:preserve` - Prevent form auto-reset
- `lvt-form:no-intercept` - Skip WebSocket, use real HTTP POST

### LiveTemplate Integration

- **Tree-based Updates**: Only changed dynamic values are sent over the wire
- **Static Content Caching**: HTML structure is cached client-side
- **Differential Updates**: Bandwidth savings of 90%+ compared to full page refreshes
- **Conditional Rendering**: Template conditionals are handled automatically

## Architecture

```
Browser                    WebSocket/HTTP              Go Server
┌─────────────────┐        ┌──────────┐               ┌──────────────────┐
│ counter.tmpl    │        │          │               │ CounterState     │
│ (rendered HTML) │        │          │               │   implements     │
│                 │        │          │               │   Store          │
│ [+1] [-1] [Reset]◄──────►│  /live   │◄─────────────►│                  │
│                 │        │          │               │ Change(action,   │
│ LiveTemplate    │        │          │               │   data)          │
│ Client JS       │        │          │               │                  │
│                 │        │          │               │ Handle()         │
│ Button name     │        │ Auto-    │               │ - Clones state   │
│ routing         │        │ detects  │               │ - Generates      │
│ (Tier 1 HTML)   │        │ transport│               │   updates        │
│                 │        │          │               │ - Publishes      │
└─────────────────┘        └──────────┘               └──────────────────┘
```

## Example Update Payloads

**Initial State (counter = 0):**
```json
{
  "s": ["<!DOCTYPE html><html>...", "...</html>"],
  "0": "Live Counter",
  "1": "0",
  "2": "zero",
  "3": "Counter is zero",
  "4": "2025-09-30 00:20:00",
  "5": "session-1727654400"
}
```

**After Increment (only changed values):**
```json
{
  "1": "1",
  "2": "positive",
  "3": "Counter is positive",
  "4": "2025-09-30 00:20:05"
}
```

This demonstrates LiveTemplate's bandwidth efficiency - subsequent updates contain only the 4 changed dynamic values instead of the full HTML document.

## Template Structure

The template follows the same pattern as `testdata/e2e/counter/input.tmpl`:

- **Title**: Dynamic page title
- **Counter Display**: Shows current counter value
- **Status**: Shows "positive", "negative", or "zero"
- **Conditional Text**: Different messages based on counter value
- **Interactive Controls**: Buttons for user actions
- **Metadata**: Last updated timestamp and session ID

## Development Notes

- **Port**: Defaults to `:8080`, can be overridden with `PORT` environment variable
- **Endpoint**: `/live` handles both WebSocket upgrades and HTTP POST requests
- **Template Path**: Reads from `examples/counter/counter.tmpl`
- **Client Library**: Serves `client/dist/livetemplate-client.browser.js` via `internal/testing.ServeClientLibrary()` (development only - use CDN in production)
- **Building Client**: Run `cd client && npm run build` to regenerate the browser bundle
- **State Isolation**: Each WebSocket connection gets its own cloned state
- **Session Management**: HTTP connections use cookie-based sessions for state persistence
- **Error Handling**: Automatic WebSocket reconnection and comprehensive error logging

## Controller+State Pattern

The counter uses the Controller+State pattern introduced in v0.7.0:

```go
// Controller: singleton, holds dependencies
controller := &CounterController{}

// State: pure data, cloned per session
initialState := &CounterState{Title: "Live Counter", Counter: 0}

tmpl := livetemplate.Must(livetemplate.New("counter"))
http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
```

- **Controller** holds dependencies (DB, Logger, etc.) — never cloned
- **State** is pure data — cloned per session, serializable
- Action methods on the controller receive state and return modified state