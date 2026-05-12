# Plan: Improve UI/UX — Todos as Showcase + Shared CSS + CSP Compliance

## Context

The examples lack visual consistency, have CSP violations (inline JS/styles), and the `livetemplate.css` in the client repo is underutilized (only 25 lines of CSS custom property defaults). The todos example should become the flagship showcase for the main README, demonstrating LiveTemplate's top features with minimal, readable code. UX extends to code readability — templates should be standard HTML with lvt attributes only when truly needed.

This plan supersedes the previous UI/UX improvement plan with the latest library capabilities (client v0.8.19), CSP compliance, and showcase-first approach.

**Prerequisites**: Before starting implementation, update all dependencies to their latest versions:
- `github.com/livetemplate/livetemplate` — latest release
- `github.com/livetemplate/lvt` — latest pseudo-version
- `@livetemplate/client` — latest release (currently v0.8.19)

Run `go get -u` in each example's `go.mod` and update CDN script tags to pin the latest client version.

---

## Progress Tracker

| # | Task | Status | Depends On |
|---|------|--------|------------|
| 1 | Extend `livetemplate.css` with semantic utilities + chat styles | ✅ | — |
| 2 | Publish CSS in client `package.json` files array | ✅ | 1 |
| 3 | Todos template: CSP fix, layout, showcase lvt-* features | ✅ | 1 |
| 4 | Todos Go code: simplify toggle, serve CSS | ✅ | 3 |
| 5 | Fix all other templates (universal + per-example) | ✅ | 1 |
| 6 | Go controller changes (notepad Change(), CSS serving) | ✅ | 5 |
| 7 | Update CLAUDE.md with new constraints | ✅ | 3, 5 |
| 8 | Update README with showcase section | ✅ | 3 |
| 9 | Run `./test-all.sh`, fix breakage | ✅ | all above |
| 10 | Record two-tab demo GIF | ⬜ | 9 |
| 11 | Clean up stale binaries (todos-components, todos-progressive, profile-progressive) | ✅ (already absent) | — |

---

## Phase 1: Shared CSS Infrastructure

### 1a. Extend `livetemplate.css` (in client repo)

Keep existing CSS custom property defaults, add semantic utilities that all examples share:

```css
/* === Existing: lvt-fx:* directive defaults === */
:root {
  --lvt-scroll-behavior: auto;
  --lvt-scroll-threshold: 100;
  --lvt-highlight-duration: 500;
  --lvt-highlight-color: #ffc107;
  --lvt-animate-duration: 300;
}

/* === Layout === */

/* Narrow container for focused, single-purpose apps */
:root {
  --pico-container-max-width: 640px;
}

/* === Utility Classes === */

/* Compact buttons/inputs for inline use in tables and toolbars */
.compact {
  --pico-form-element-spacing-vertical: 0.25rem;
  --pico-form-element-spacing-horizontal: 0.5rem;
  font-size: 0.875rem;
  width: auto;
}

/* Visually hidden but accessible to screen readers */
.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

/* Zero-margin form for embedding in tables/toolbars */
.inline {
  margin: 0;
}

/* === Chat / Message List === */

/* Scrollable message container */
.messages {
  border: 1px solid var(--pico-muted-border-color);
  border-radius: var(--pico-border-radius);
  padding: 1rem;
  height: 400px;
  overflow-y: auto;
  background: var(--pico-card-background-color);
  margin-bottom: 1rem;
}

/* Individual message card */
.message {
  padding: 0.75rem;
  margin-bottom: 0.5rem;
  border-radius: var(--pico-border-radius);
  border-left: 3px solid var(--pico-muted-border-color);
  background: var(--pico-background-color);
}

/* Own message highlight */
.message.mine {
  background: var(--pico-primary-background);
  border-left-color: var(--pico-primary);
}
```

### 1b. Update client `package.json`

Add `"livetemplate.css"` to the `files` array so it's published to npm:

```json
"files": [
  "dist/**/*",
  "livetemplate.css",
  "README.md",
  "LICENSE"
]
```

### 1c. Serving pattern for examples

Each example's `main.go` serves the CSS locally. Add a helper in `e2etest` or use `http.ServeFile`:

```go
http.HandleFunc("/livetemplate.css", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "../client/livetemplate.css")  // relative to example dir
})
```

Templates link it conditionally:
```html
{{ if .lvt.DevMode }}
  <link rel="stylesheet" href="/livetemplate.css">
{{ else }}
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/livetemplate.css">
{{ end }}
```

---

## Phase 2: Todos as Showcase

### Critical files
- `todos/todos.tmpl`
- `todos/controller.go`
- `todos/state.go`
- `todos/main.go`
- `todos/todos_test.go`

### 2a. CSP fix — checkbox toggle (line 83 of todos.tmpl)

**Before** (CSP violation):
```html
<form method="POST" name="toggle">
  <input type="hidden" name="id" value="{{ .ID }}" />
  <input type="checkbox" {{ if .Completed }}checked{{ end }}
    onchange="this.form.requestSubmit()" aria-label="Toggle completion" />
  <button type="submit" name="toggle" hidden>Toggle</button>
</form>
```

**After** (CSP-safe, using existing `lvt-on:change`):
```html
<input type="checkbox"
  {{ if .Completed }}checked{{ end }}
  lvt-on:change="toggle"
  data-id="{{ .ID }}"
  aria-label="Toggle completion" />
```

The entire form wrapper, hidden input, and hidden button are eliminated. The `data-id` standard attribute passes the todo ID to the `Toggle()` action via `ctx.GetString("id")`.

**Controller change** (`controller.go` Toggle method): Update to read `id` from data attributes instead of form data. The `lvt-on:change` event delegation collects `data-*` attributes automatically.

### 2b. HTML hygiene

1. Remove `data-theme="light"` from `<html>` tag
2. Add `<meta name="color-scheme" content="light dark">` to `<head>` — enables automatic dark mode via Pico CSS
3. Remove entire `<style>` block — `.visually-hidden` now in shared CSS
4. Link shared CSS in `<head>`
5. Standardize `<title>` to `Todo App — LiveTemplate`
6. Verify `lang="en"` is present

### 2c. Layout improvements

1. **Stats into hgroup subtitle**: Move stats from separate `{{ template "stats" }}` into the article header as part of `<hgroup>`:
   ```html
   <hgroup>
     <h1>{{ .Title }}</h1>
     <p>Total: <strong>{{ .TotalCount }}</strong> · Completed: <strong>{{ .CompletedCount }}</strong> · Remaining: <strong>{{ .RemainingCount }}</strong></p>
   </hgroup>
   ```

2. **Combine search + sort into one row**:
   ```html
   <div class="grid">
     {{ template "search-form" . }}
     {{ template "sort-select" . }}
   </div>
   ```

3. **Compact delete buttons**: `class="compact secondary outline"` on delete buttons in table
4. **Compact pagination**: `class="compact"` on prev/next buttons
5. **Compact clear completed**: `class="compact secondary outline"` instead of full-width
6. **Inline forms in table cells**: `class="inline"` on forms inside `<td>`

### 2d. Showcase features (Tier 2 lvt-* attributes)

These demonstrate LiveTemplate's key capabilities with minimal code:

1. ~~**Form reset on success**~~ — NOT NEEDED: forms auto-reset after successful submission (Tier 1 behavior). Use `lvt-form:preserve` to opt out.

2. **Entry animations** — new todo rows fade in:
   ```html
   <tr data-key="{{ .ID }}" lvt-fx:animate="fade">
   ```

3. **Highlight on toggle** — visual flash when a todo's status changes:
   ```html
   <tr data-key="{{ .ID }}" lvt-fx:animate="fade" lvt-fx:highlight="flash">
   ```

4. **Loading states on buttons** — aria-busy during server round-trip:
   ```html
   <button type="submit" name="add"
     lvt-el:setAttr:on:add:pending="aria-busy:true"
     lvt-el:setAttr:on:add:done="aria-busy:false"
     {{.lvt.AriaDisabled "text"}}>Add</button>
   ```

5. **Dark mode** — automatic via `<meta name="color-scheme" content="light dark">` + removing `data-theme="light"`. No code changes needed; Pico CSS handles the OS preference.

### 2e. Revised todos.tmpl template sketch

The final template should demonstrate these features in this Tier breakdown:

**Tier 1 (standard HTML):**
- `<form method="POST" name="add">` → Add() action
- `<button name="confirmDelete">` → ConfirmDelete() action
- `<button name="clearCompleted">` → ClearCompleted() action
- `<button name="prevPage">` / `<button name="nextPage">` → pagination
- `<input type="search" name="query">` → Change() for live search
- `<select name="sort_by">` → Change() for live sort
- `{{ .lvt.ErrorTag "text" }}`, `{{ .lvt.AriaInvalid "text" }}`, `{{ .lvt.AriaDisabled "text" }}` → validation
- `{{ template "lvt:modal:confirm:v1" }}`, `{{ template "lvt:toast:container:v1" }}` → components
- `{{ .lvt.DevMode }}` → conditional script/CSS loading

**Tier 2 (lvt-* attributes, used sparingly):**
- `lvt-on:change="toggle"` — checkbox toggle without inline JS (CSP fix)
- `lvt-fx:animate="fade"` — entry animation on new rows
- `lvt-fx:highlight="flash"` — visual feedback on toggled rows
- `lvt-el:setAttr:on:add:pending="aria-busy:true"` — loading state on Add button
- `lvt-el:setAttr:on:add:done="aria-busy:false"` — clear loading state

This is 5 lvt attributes total, each solving a real interaction problem that standard HTML cannot express.

### 2f. New in v0.8.19: `data-lvt-target` for cross-element targeting

The client v0.8.19 adds `data-lvt-target` — `lvt-el:` methods can operate on a *different* element:
- `data-lvt-target="#id"` → targets `document.getElementById(id)`
- `data-lvt-target="closest:selector"` → targets `element.closest(selector)`

**Potential use in todos**: A "focus search" keyboard shortcut could use this. However, since focusing an input isn't an `lvt-el:` method (it's `setAttr`, `addClass`, etc.), this is better left as a future enhancement unless a `focus` method is added to `lvt-el:`. For now, document `data-lvt-target` in the Tier 2 table as available.

**Useful for other examples**: The login example could use `data-lvt-target` for modal/dialog interactions without server round-trips. These are optional enhancements beyond this plan's scope.

---

## Phase 3: Fix All Other Examples

### Universal fixes (all 9 remaining templates)

1. Add `lang="en"` to `<html>` where missing (counter, flash-messages, progressive-enhancement, ws-disabled, shared-notepad, live-preview)
2. Add `<meta name="color-scheme" content="light dark">` to all `<head>` blocks
3. Link shared CSS (`/livetemplate.css`) in all templates
4. Standardize `<title>` to `Name — LiveTemplate` format
5. Use `<hgroup>` for title + subtitle combinations
6. Wrap content in `<article>` for consistent card structure where missing
7. Serve `/livetemplate.css` in all `main.go` files

### Per-example fixes

**counter/counter.tmpl:**
- Use `<fieldset role="group">` for buttons instead of `<div class="grid">`
- Add `<hgroup>` with subtitle "Real-time counter with WebSocket"
- Make counter value prominent: `<data>{{.Counter}}</data>`

**chat/chat.tmpl:**
- Remove entire `<style>` block (~50 lines) — all styles now in shared CSS
- Replace `.input-form` grid → `<fieldset role="group">` for input+send
- Replace `.stats` div → `<small>` element
- Replace `.empty-state` div → `<p><small>...</small></p>`
- Wrap in `<article>` with `<hgroup>` (title + online count)
- Remove `body { padding: 1rem; }` — container handles this

**flash-messages/flash.tmpl:**
- Use `<fieldset role="group">` for input + Add button
- Use `<nav>` or `<fieldset role="group">` for Clear All + Simulate Error (compact)
- Add `class="compact secondary outline"` to remove buttons
- Move info blockquote into `<details>` accordion
- Add `<hgroup>` with title + subtitle

**progressive-enhancement/progressive-enhancement.tmpl:**
- **Remove entire `<style>` block** (`.js-mode`, `.completed`, `.empty-state`, `.todo-item` classes)
- **Remove inline `<script>` block** (lines 94-97) — CSP violation
- Remove JS mode indicator markup entirely; keep only `<noscript>` block
- Convert todo items to `<table>` layout (rows: checkbox | title | date | delete)
- Replace `.completed` class → `<s>` element for completed items
- Replace `.empty-state` → `<p><small>...</small></p>`
- Use `<fieldset role="group">` for add form
- Add `class="compact"` to toggle/delete buttons

**ws-disabled/ws-disabled.tmpl:**
- **Remove all 5 inline `style` attributes** (CSP violation)
- Convert bookmarks to `<table>` layout (rows: link+URL | delete)
- Use `class="compact contrast outline"` on delete buttons
- Use `class="inline"` on delete forms
- Empty state: `<p><small>No bookmarks yet.</small></p>`

**shared-notepad/notepad.tmpl:**
- **Remove `oninput="..."`** inline JS handler — CSP violation
- Add `Change()` method to Go controller for reactive char count (server-side)
- The existing `lvt-form:preserve` stays (Tier 2, genuinely needed)
- Add `lang="en"`, color-scheme meta

**avatar-upload/avatar-upload.tmpl:**
- **Fix outdated footer**: Remove "LiveTemplate v0.3.0" reference
- Add color-scheme meta

**live-preview/preview.tmpl:**
- Add `lang="en"`, color-scheme meta
- Add `<hgroup>` with subtitle "Type to see a live preview"

**login/templates/auth.html:**
- Add color-scheme meta
- Make logout button `class="compact contrast"` (secondary action)

---

## Phase 4: Go Controller Changes

### 4a. shared-notepad controller

Add `Change()` method to replace inline JS char count:
```go
func (c *NotepadController) Change(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
    if ctx.Has("content") {
        state.Content = ctx.GetString("content")
        state.CharCount = len([]rune(state.Content))
    }
    return state, nil
}
```

### 4b. todos controller

Update `Toggle()` to work with `lvt-on:change` data attributes:
- The `data-id` attribute gets collected by the event delegator as `id` in the action data
- `ctx.GetString("id")` continues to work
- Remove the `ToggleInput` struct usage if form data shape changes
- The checkbox's `checked` state is NOT sent — Toggle() flips the DB value, same as before

### 4c. All examples — serve CSS

Each `main.go` gets:
```go
http.HandleFunc("/livetemplate.css", e2etest.ServeCSS) // or ServeFile
```

If `e2etest.ServeCSS` doesn't exist, add it to the testing package alongside `ServeClientLibrary`, pointing to the client package's `livetemplate.css`.

---

## Phase 5: Update CLAUDE.md

### Add: CSP Compliance (new section, critical constraint)

```
## CSP Compliance

All templates must be compatible with `Content-Security-Policy: script-src 'self'`:

- NEVER use inline event handlers (`onchange`, `oninput`, `onclick`, etc.)
- NEVER use `<style>` blocks in templates — all CSS in `livetemplate.css`
- NEVER use inline `<script>` blocks (except the conditional client library loader)
- NEVER use inline `style` attributes (except `<ins>`/`<del>` block pattern)
- Use `lvt-on:{event}` attributes instead of inline JS event handlers
- Use `Change()` method for reactive input handling
```

### Add: Shared CSS section

```
## Shared CSS

All examples link `livetemplate.css` for shared utilities:
- Served at `/livetemplate.css` (dev) or CDN (production)
- Contains: narrow container (640px), `.compact` buttons, `.visually-hidden`,
  `.inline` forms, `.messages`/`.message` for chat-style UIs
- Do NOT add example-specific CSS to livetemplate.css — only reusable patterns
```

### Add: HTML Boilerplate Standard

```
## HTML Boilerplate

Every template must use this boilerplate:

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <title>App Name — LiveTemplate</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
    {{ if .lvt.DevMode }}
    <link rel="stylesheet" href="/livetemplate.css">
    {{ else }}
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/livetemplate.css">
    {{ end }}
</head>
```

### Add: Visual Layout Standard

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
```

### Update: Tier 2 Constructs table

Add to existing table:
```
| `lvt-on:change` | Checkbox/input change without inline JS | Todo toggle |
| `lvt-el:reset:on:{action}:success` | Form auto-clear on success | Add form |
| `lvt-el:setAttr:on:{action}:pending` | Loading state during round-trip | Submit button |
| `data-lvt-target` | Cross-element targeting for lvt-el: methods | Modal open/close, dropdown toggle |
```

---

## Phase 6: README Showcase

Add a showcase section at the top of `README.md`, before the Progressive Complexity table:

```markdown
## Showcase: Todo App

![Todo App Demo](docs/assets/todos-demo.gif)

The todo app demonstrates LiveTemplate's core features in ~150 lines of Go + ~80 lines of HTML:

- **Real-time sync** — open two tabs as the same user; changes appear instantly via explicit `BroadcastAction`
- **Standard HTML forms** — `<form method="POST" name="add">` routes to `Add()` with zero configuration
- **Live search & sort** — `Change()` auto-wires input events with 300ms debounce
- **Validation** — `ErrorTag`, `AriaInvalid`, `AriaDisabled` template helpers
- **Components** — modal confirmation dialogs and toast notifications
- **Entry animations** — `lvt-fx:animate="fade"` on new rows
- **Loading states** — `lvt-el:setAttr:on:pending="aria-busy:true"` for visual feedback
- **Dark mode** — automatic via `<meta name="color-scheme" content="light dark">`
- **Progressive enhancement** — works without JavaScript via HTTP POST fallback

**Left tab**: Add, search, sort, and paginate todos
**Right tab**: Same user — watch changes sync in real-time
```

### Two-tab demo GIF

Record with screen capture (Kap, ffmpeg, or similar):
1. Split screen: two browser windows side by side, both at `localhost:8080`
2. Both logged in as `alice`
3. Demo sequence:
   - Tab A: Add "Buy groceries" → appears in Tab B via BroadcastAction, fade animation
   - Tab B: Toggle complete → strikethrough appears in Tab A, highlight flash
   - Tab A: Search "buy" → filters in Tab A only (independent per-tab)
   - Tab B: Delete with modal confirmation → disappears from Tab A
   - Show toast notifications appearing in both tabs
   - Toggle dark mode (OS preference or browser devtools)

Save GIF to `docs/assets/todos-demo.gif`.

---

## Phase 7: Tests

### Test adjustments expected

**todos/todos_test.go** (1213 lines):
- Checkbox toggle: selectors change from `form[name=toggle] input[type=checkbox]` to `input[type=checkbox][lvt-on\\:change]` or similar
- If stats move into hgroup, update text matching
- If delete buttons get `class="compact"`, selectors still work (name-based)
- Pagination button selectors still work (name-based: `button[name=prevPage]`)
- Form reset behavior: verify input clears after add

**Other tests**: Selector changes for restructured HTML (div → table, class removal, etc.)

### Verification

```bash
# Run full test suite
./test-all.sh

# Individual example testing during development
cd todos && go test -v -run TestTodosE2E
cd chat && go test -v -run TestChatE2E
# ... etc for each modified example
```

---

## Phase 8: Cleanup

- Remove stale binary directories: `todos-components/`, `todos-progressive/`, `profile-progressive/`
- Verify `.gitignore` excludes binary artifacts

---

## New Constraints for Future Examples (CLAUDE.md additions summary)

1. **CSP compliance is mandatory** — no inline JS, no inline styles, no `<style>` blocks
2. **All CSS goes in shared `livetemplate.css`** — if a new pattern is needed, add it there; never in templates
3. **Dark mode by default** — always include `<meta name="color-scheme" content="light dark">`
4. **Use `lvt-on:change` for immediate input responses** instead of inline `onchange`/`oninput`
5. **Use `Change()` for debounced input responses** instead of inline JS
6. **Use `lvt-el:` for client-side reactions** (form reset, loading states) instead of custom JS
7. **Use `lvt-fx:` for visual polish** (animations, highlights) instead of CSS animations
8. **Standard HTML boilerplate** — lang, color-scheme, title format, shared CSS link are all mandatory

---

## New lvt Attribute Suggestions

No new attributes are needed for the current plan. All CSP violations are solvable with existing attributes:

| CSP Violation | Solution | Existing Attribute |
|---|---|---|
| `onchange="this.form.requestSubmit()"` | `lvt-on:change="toggle"` + `data-id` | `lvt-on:{event}` (v0.8.13+, data-* collection v0.8.19) |
| `oninput="..."` for char count | `Change()` method (server-side) | Auto-wired by framework |
| Inline `<script>` for mode indicator | Remove indicator; keep `<noscript>` | N/A — just remove the JS |

**Future consideration**: If more examples need "auto-submit on input change" (form-level, not action-level), consider `lvt-form:auto-submit` as a convenience attribute. But for now, `lvt-on:change` covers the use case.

---

## New Semantic CSS Suggestions for livetemplate.css

Beyond what's in Phase 1, consider these for future additions (not in this plan's scope):

| Pattern | CSS | Use Case |
|---|---|---|
| `.truncate` | `overflow: hidden; text-overflow: ellipsis; white-space: nowrap;` | Long text in table cells |
| `.fade-in` | `animation: lvt-fade-in var(--lvt-animate-duration, 300ms)` | Manual fade-in without `lvt-fx:animate` |
| `.message-header` | `font-size: 0.85rem; margin-bottom: 0.25rem; opacity: 0.8;` | Chat message headers |
| `.message-time` | `float: right;` | Timestamp alignment in messages |

These can be added later as needed. The Phase 1 CSS covers all immediate needs.

---

## Implementation Order (recommended)

1. **Start in client repo**: extend CSS, update package.json
2. **Todos example**: template + controller + main.go + tests — get the showcase working first
3. **Other examples**: apply universal fixes, then per-example fixes, test each
4. **CLAUDE.md + README**: update docs
5. **Demo GIF**: record after everything passes
6. **Cleanup**: remove stale binaries
