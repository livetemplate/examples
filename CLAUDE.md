# LiveTemplate Examples

## Progressive Complexity

All examples follow the **progressive complexity** model introduced in livetemplate v0.8.7:

- **Tier 1 (Standard HTML)** is the default. Use native HTML forms, buttons, and inputs.
- **Tier 2 (`lvt-*` attributes)** only when standard HTML cannot express the behavior.

### Tier 1 Constructs

| Construct | Pattern | Routes to |
|-----------|---------|-----------|
| Form submission | `<form method="POST" name="add">` | `Add()` method |
| Button action | `<button name="save">` | `Save()` method |
| Hidden data | `<input type="hidden" name="id" value="...">` | `ctx.GetString("id")` |
| Auto-submit | `<form method="POST">` (no name) | `Submit()` method |
| Live updates | Controller with `Change()` method | Auto-wired 300ms debounce |
| Form auto-reset | Forms reset after successful submission | Use `lvt-form:preserve` to retain values |
| Validation | `required`, `minlength`, `pattern` | `ctx.ValidateForm()` |

### Tier 2 Constructs (use sparingly)

| Attribute | Purpose | Example |
|-----------|---------|---------|
| `lvt-fx:scroll` | Auto-scroll behavior | Chat message container |
| `lvt-upload` | Chunked file uploads | Avatar upload |
| `lvt-mod:debounce` | Custom timing control | Search with custom delay |
| `lvt-on:keydown` | Keyboard shortcuts | Global key bindings |
| `lvt-on:change` | Checkbox/input change without inline JS | Todo toggle |
| `lvt-fx:animate` | Entry/exit animations | Toast notifications, todo rows |
| `lvt-fx:highlight` | Visual flash on change | Toggled todo rows |
| `lvt-el:setAttr:on:{action}:pending` | Loading state during round-trip | Submit button |
| `lvt-form:preserve` | Preserve form state across re-renders | Shared notepad |
| `lvt-form:no-intercept` | Skip WebSocket, use real HTTP POST | Login/logout forms |
| `data-lvt-target` | Cross-element targeting for lvt-el: methods | Modal open/close |

### Action Resolution Order

When a form is submitted, the framework resolves the action in this order:
1. Clicked button's `name` attribute
2. Form's `name` attribute
3. Default: `"submit"`

## Creating New Examples

### Checklist

1. Start with Tier 1 — use standard HTML forms and button names
2. Only add `lvt-*` attributes when standard HTML can't express the interaction
3. Add `method="POST"` to forms and `name` attributes for action routing
4. Use form `name` for both client-side (WebSocket/fetch) and server-side (HTTP POST) action routing
5. Use button `name` for HTTP POST fallback (browser includes button name in form data on click)
6. Add hidden inputs for data passing: `<input type="hidden" name="id" value="{{.ID}}">`

### Testing

- All examples must have chromedp E2E tests
- Use `e2etest.StartDockerChrome()` for browser testing
- For WebSocket CRUD verification, use `window.liveTemplateClient.send({action: '...', data: {...}})` directly
- HTTP POST tests should use button name encoding: `"add=&field=value"`
- Run `./test-all.sh` to verify all examples pass

### Dependencies

- livetemplate: v0.8.7+
- lvt (testing): latest pseudo-version
- Client library: served via `e2etest.ServeClientLibrary` (dev) or CDN (production)

### Reference Examples

- `todos/` — Canonical Tier 1 example: CRUD, auth, pagination, modal + toast components
- `live-preview/` — Tier 1 with `Change()` method for live updates
- `chat/` — Tier 1+2 (uses `lvt-fx:scroll` for auto-scroll)

## Framework Documentation

Before writing code, always consult the LiveTemplate reference docs and guides:

- **References:** `https://github.com/livetemplate/livetemplate/tree/main/docs/references/` — client attributes, server API, action routing
- **Guides:** `https://github.com/livetemplate/livetemplate/tree/main/docs/guides/` — progressive complexity, patterns, best practices
- **Ephemeral Components:** `https://github.com/livetemplate/livetemplate/tree/main/docs/guides/ephemeral-components.md` — implementing client-side toasts, alerts, and banners without server diffs

Use framework-native solutions instead of custom JavaScript. Common patterns:
- `input type="search"` has a browser-native clear button; the framework handles the `search` event automatically (no custom JS needed)
- The `Change()` method auto-wires input/change/search events on named form fields with 300ms debounce
- Use `hidden` HTML attribute for visibility toggling (not `style="display:none"`)

## CSP Compliance

All templates must be compatible with `Content-Security-Policy: script-src 'self'`:

- NEVER use inline event handlers (`onchange`, `oninput`, `onclick`, etc.)
- NEVER use `<style>` blocks in templates — all CSS in `livetemplate.css`
- NEVER use inline `<script>` blocks (except the conditional client library loader)
- NEVER use inline `style` attributes (except `<ins>`/`<del>` block pattern)
- Use `lvt-on:{event}` attributes instead of inline JS event handlers
- Use `Change()` method for reactive input handling

## Shared CSS

All examples link `livetemplate.css` for shared utilities:
- Served at `/livetemplate.css` (dev) or CDN (production)
- Contains: narrow container (640px), `.compact` buttons, `.visually-hidden`, `.inline` forms, `.messages`/`.message` for chat-style UIs
- Do NOT add example-specific CSS to livetemplate.css — only reusable patterns

## HTML Boilerplate

Every template must use this boilerplate:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <title>App Name — LiveTemplate</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
    {{if .lvt.DevMode}}
    <link rel="stylesheet" href="/livetemplate.css">
    {{else}}
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/livetemplate.css">
    {{end}}
</head>
```

## Visual Layout

- Wrap content in `<main class="container">` (640px via shared CSS)
- Use `<article>` as primary card container
- Use `<hgroup>` for title + subtitle
- Group related controls: `<fieldset role="group">` or `<div class="grid">`
- Inline forms in tables: `class="inline"`
- Compact action buttons: `class="compact secondary"` or `class="compact contrast outline"`
- Empty states: `<p><small>Message</small></p>`
- Completed items: `<s>` element (not CSS classes)

## CSS

All examples use [Pico CSS](https://picocss.com/docs) exclusively:

- Include via CDN: `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">`
- Use semantic HTML — Pico auto-styles: `<article>` (cards), `<dialog>` (modals), `<details>` (accordions), `<table>`, `<nav>`, `<progress>`
- Use Pico classes sparingly: `.container`, `.grid`, `.secondary`, `.contrast`, `.outline`
- Use `aria-invalid="true"` for form validation errors, `<small>` for helper/error text
- Use `<ins>` for success messages, `<del>` for error messages, using the standardized inline style `style="display:block;text-decoration:none"` when rendering them as block-level alerts
- Use `<s>` for strikethrough text (e.g., completed todos), `<del>` for removed/error content
- Use `<mark>` for highlighted/badge text
- Use `<progress>` for progress bars
- Use `<hgroup>` for title + subtitle groupings
- Use `<fieldset role="group">` for inline input+button groups
- Use `<blockquote>` for callout/info boxes
- Do NOT write inline `style` attributes, except for the standardized `<ins>`/`<del>` block-level pattern above. Use Pico semantic elements instead (e.g., `<s>` not `style="text-decoration:line-through"`, `<nav>` not `style="display:flex"`, `hidden` not `style="display:none"`)
- Do NOT write custom CSS. If Pico cannot express a style, ask before adding custom CSS.
- Pico CSS variables (`--pico-*`) may be used for theming when semantic markup is insufficient
