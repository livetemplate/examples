# Todo App — Progressive Complexity (Tier 1)

A todo application built with **zero `lvt-*` attributes**. All action routing uses standard HTML.

This demonstrates LiveTemplate's [progressive complexity model](https://github.com/livetemplate/livetemplate/blob/main/docs/guides/progressive-complexity.md) — Tier 1: Standard HTML.

## What Makes This Different

Compare with the [standard todos example](../todos/) which uses `lvt-submit` and `lvt-click`. This version achieves the same functionality with pure HTML:

| Feature | Standard Example | This Example |
|---------|-----------------|--------------|
| Add todo | `<form lvt-submit="add">` | `<form method="POST">` (auto-routes to `Submit()`) |
| Toggle done | `<button lvt-click="toggle">` | `<button name="toggle">` |
| Delete | `<button lvt-click="delete">` | `<button name="delete">` |
| Filter | `lvt-click` buttons | `<form name="filter">` |
| Data passing | `lvt-data-id="{{.ID}}"` | `<input type="hidden" name="id">` |

## How It Works

**Button name IS the action.** `<button name="toggle">` routes to the `Toggle()` method on the controller. No custom attributes needed.

**Three transport levels — same template:**
- **No JavaScript:** Standard POST + page reload (progressive enhancement)
- **JS + HTTP:** `fetch()` POST + DOM patching (no page reload)
- **JS + WebSocket:** Real-time updates + server push

## Running

```bash
cd todos-progressive
go run main.go
```

Open http://localhost:8080

## Key Patterns

### Auto-submit (no button name)
```html
<form method="POST">
    <input name="Title" placeholder="New todo...">
    <button type="submit">Add</button>
</form>
```
Routes to `Submit()` — the conventional default.

### Button name routing
```html
<button name="toggle">Done</button>
<button name="delete">Delete</button>
```
Routes to `Toggle()` and `Delete()`.

### Form name routing
```html
<form name="filter" method="POST">
    <button name="filter" value="all">All</button>
</form>
```
Routes to `Filter()`.

### Data passing via hidden inputs
```html
<input type="hidden" name="id" value="{{.ID}}">
```
Accessed in Go via `ctx.GetString("id")`.

## When to Add `lvt-*` (Tier 2)

This example stays in Tier 1. You'd reach for Tier 2 when you need:
- `lvt-debounce` — wait for typing pause (e.g., live search)
- `lvt-key="Enter"` — keyboard shortcut filtering
- `lvt-addClass-on:pending` — reactive DOM during submission
- `lvt-hook` — integrating JavaScript libraries (charts, maps)
