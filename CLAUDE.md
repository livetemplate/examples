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
| Validation | `required`, `minlength`, `pattern` | `ctx.ValidateForm()` |

### Tier 2 Constructs (use sparingly)

| Attribute | Purpose | Example |
|-----------|---------|---------|
| `lvt-scroll` | Auto-scroll behavior | Chat message container |
| `lvt-upload` | Chunked file uploads | Avatar upload |
| `lvt-debounce` | Custom timing control | Search with custom delay |
| `lvt-keydown` | Keyboard shortcuts | Global key bindings |
| `lvt-animate` | Entry/exit animations | Toast notifications |

### Action Resolution Order

When a form is submitted, the framework resolves the action in this order:
1. `lvt-submit` attribute on the form
2. Clicked button's `name` attribute
3. Form's `name` attribute
4. Default: `"submit"`

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

- `todos-progressive/` — Canonical Tier 1 example (zero `lvt-*` attributes)
- `profile-progressive/` — Simple Tier 1 form with validation
- `live-preview/` — Tier 1 with `Change()` method for live updates
- `chat/` — Tier 1+2 (uses `lvt-scroll` for auto-scroll)

## Framework Documentation

Before writing code, always consult the LiveTemplate reference docs and guides:

- **References:** `https://github.com/livetemplate/livetemplate/docs/references/` — client attributes, server API, action routing
- **Guides:** `https://github.com/livetemplate/livetemplate/docs/guides/` — progressive complexity, patterns, best practices

Use framework-native solutions instead of custom JavaScript. Common patterns:
- `input type="search"` has a browser-native clear button; the framework handles the `search` event automatically (no custom JS needed)
- The `Change()` method auto-wires input/change/search events on named form fields with 300ms debounce
- Use `hidden` HTML attribute for visibility toggling (not `style="display:none"`)

## CSS

All examples use [Pico CSS](https://picocss.com/docs) exclusively:

- Include via CDN: `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">`
- Use semantic HTML — Pico auto-styles: `<article>` (cards), `<dialog>` (modals), `<details>` (accordions), `<table>`, `<nav>`, `<progress>`
- Use Pico classes sparingly: `.container`, `.grid`, `.secondary`, `.contrast`, `.outline`
- Use `aria-invalid="true"` for form validation errors, `<small>` for helper/error text
- Use `<ins>` for success messages, `<del>` for error messages (with `style="display:block;text-decoration:none"`)
- Use `<s>` for strikethrough text (e.g., completed todos), `<del>` for removed/error content
- Use `<mark>` for highlighted/badge text
- Use `<progress>` for progress bars
- Use `<hgroup>` for title + subtitle groupings
- Use `<fieldset role="group">` for inline input+button groups
- Use `<blockquote>` for callout/info boxes
- Do NOT write inline `style` attributes. Use Pico semantic elements instead (e.g., `<s>` not `style="text-decoration:line-through"`, `<nav>` not `style="display:flex"`, `hidden` not `style="display:none"`)
- Do NOT write custom CSS. If Pico cannot express a style, ask before adding custom CSS.
- Pico CSS variables (`--pico-*`) may be used for theming when semantic markup is insufficient
