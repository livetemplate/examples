# Progressive Enhancement Example

This example demonstrates how LiveTemplate supports progressive enhancement - allowing apps to work both with and without JavaScript enabled.

## How It Works

### With JavaScript (WebSocket Mode)
- Actions are sent via WebSocket for instant updates
- No page reloads - UI updates in real-time
- Best user experience for modern browsers

### Without JavaScript (HTTP Form Mode)
- Actions are submitted via standard HTML forms
- Server returns full HTML pages using POST-Redirect-GET pattern
- Page reloads after each action
- Works on any browser, including text-based browsers

## Key Concepts

### Dual-Mode Forms

Forms work in both modes using standard HTML with `method="POST"` and button `name` routing:

```html
<form method="POST" name="add">
    <input type="text" name="title">
    <button type="submit" name="add">Add</button>
</form>
```

- **With JS**: The client intercepts the form and routes via WebSocket/fetch
- **Without JS**: Standard form submission sends POST request to server

### POST-Redirect-GET (PRG) Pattern

For non-JS clients, successful actions redirect using HTTP 303:

1. User submits form via POST
2. Server processes action, updates state
3. Server responds with 303 redirect to same URL
4. Browser follows redirect with GET
5. User sees updated page

This prevents duplicate submissions when users refresh the page.

### Validation Errors

When validation fails:
- **With JS**: Errors appear instantly via WebSocket update
- **Without JS**: Server re-renders the page with errors inline (no redirect)

### Flash Messages

Success/error messages are shown once after actions:
- **With JS**: Messages appear in real-time
- **Without JS**: Messages passed via query params after redirect

## Running the Example

```bash
# Development mode (uses local client library)
LVT_DEV_MODE=true go run .

# Production mode (uses CDN client library)
go run .
```

Visit http://localhost:8080 and try:
1. With JavaScript enabled - notice instant updates
2. Disable JavaScript and refresh - notice page reloads after each action
3. Both modes provide the same functionality

## Configuration

Progressive enhancement is enabled by default. To disable it:

```go
// Via environment variable
LVT_PROGRESSIVE_ENHANCEMENT=false go run .

// Or via code
tmpl := livetemplate.New("app", livetemplate.WithProgressiveEnhancement(false))
```

## Files

- `main.go` - Controller, state, and action handlers
- `progressive-enhancement.tmpl` - Template with dual-mode forms
- `progressive_enhancement_test.go` - End-to-end tests for progressive enhancement behavior
- `README.md` - This documentation
