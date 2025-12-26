# Flash Messages Example

This example demonstrates flash messages in LiveTemplate - page-level notifications that show once and clear after each action.

## Running

```bash
cd examples/flash-messages
go run .
```

Then open http://localhost:8080

## Flash Message Types

| Type | Use Case | Style |
|------|----------|-------|
| `success` | Operation completed | Green |
| `error` | Something went wrong | Red |
| `warning` | Caution/duplicate | Yellow |
| `info` | Informational | Blue |

## Setting Flash Messages (Controller)

```go
func (c *Controller) MyAction(state State, ctx *livetemplate.Context) (State, error) {
    // Success notification
    ctx.SetFlash("success", "Item added successfully!")

    // Error notification
    ctx.SetFlash("error", "Failed to save changes")

    // Warning notification
    ctx.SetFlash("warning", "Item already exists")

    // Info notification
    ctx.SetFlash("info", "Processing complete")

    return state, nil
}
```

## Reading Flash Messages (Template)

```html
<!-- Check if any flash exists -->
{{if .lvt.HasAnyFlash}}
<div id="flash-messages">

    <!-- Check specific flash type -->
    {{if .lvt.HasFlash "success"}}
    <div class="alert alert-success">{{.lvt.Flash "success"}}</div>
    {{end}}

    {{if .lvt.HasFlash "error"}}
    <div class="alert alert-error">{{.lvt.Flash "error"}}</div>
    {{end}}

</div>
{{end}}
```

## Flash vs Field Errors

| Aspect | Flash Messages | Field Errors |
|--------|----------------|--------------|
| **Purpose** | Page-level notifications | Form field validation |
| **Affects Success** | No | Yes |
| **Template Access** | `.lvt.Flash "key"` | `.lvt.Error "field"` |
| **Lifecycle** | Cleared after render | Cleared on next action |
| **Example** | "Changes saved!" | "Email is required" |

## Key Behaviors

1. **Show Once**: Flash messages are cleared after each action response
2. **Per-Connection**: Not shared across browser tabs
3. **No Persistence**: Don't survive page refresh or WebSocket reconnects
4. **Don't Block Success**: Unlike field errors, flash messages don't set `Success: false`

## Available Template Helpers

| Helper | Description |
|--------|-------------|
| `.lvt.Flash "key"` | Get flash message for key |
| `.lvt.HasFlash "key"` | Check if flash exists for key |
| `.lvt.HasAnyFlash` | Check if any flash messages exist |
| `.lvt.AllFlash` | Get all flash messages as map |
