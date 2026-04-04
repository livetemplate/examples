# Plan: Improve UI/UX of All Examples (Issue #51)

## Problem

The examples lack visual polish and consistency. Content stretches too wide, buttons are oversized for their context, controls are spread across multiple rows wasting vertical space, and each example feels like it was built by a different person. Additionally, there are inline styles/JS violating CLAUDE.md and missing accessibility attributes. The goal is to produce cohesive, production-quality examples with tight visual design and establish guidelines for future apps.

## Approach

1. Create a shared `livetemplate.css` with visual utilities (narrow container, compact buttons, visually-hidden)
2. Apply visual improvements to every template: narrow focused layout, compact action buttons, single-row toolbars, breathing space
3. Clean up code hygiene: remove inline styles/JS, fix accessibility
4. Update CLAUDE.md with established visual and coding standards
5. Verify all tests still pass

---

## Visual Design Principles

These apply to ALL examples:

1. **Narrow, focused layout** — All examples are single-purpose apps. Constrain content to ~640px max-width for a focused feel with breathing space on wider screens.
2. **Single-row toolbars** — Group related controls (search + sort, action buttons) into one horizontal row using `<fieldset role="group">` or `<nav>`.
3. **Compact action buttons** — Inline action buttons (delete, toggle, remove) should be small, not full-height Pico buttons. Use a `.compact` utility class.
4. **Consistent card structure** — Every example: `<article>` card with `<hgroup>` header (title + subtitle), content, optional footer.
5. **Visual hierarchy** — Primary actions get prominent buttons; secondary/destructive actions get `outline` or `secondary` styling and smaller size.
6. **Breathing space** — Let Pico's natural spacing work; don't stack multiple full-width elements unnecessarily.

---

## Phase 1: Create Shared CSS (`livetemplate.css`)

Create `../client/livetemplate.css` (relative to examples root) with reusable utilities:

```css
/* Narrow container for focused, single-purpose apps */
:root {
  --pico-container-max-width: 640px;
}

/* Compact action buttons for inline use in tables and toolbars */
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

/* Inline form — no margin, for embedding in tables/toolbars */
.inline {
  margin: 0;
}
```

**Rationale**: These four utilities address the core visual problems:
- `--pico-container-max-width: 640px` → narrow focused layout (breathing space on wide screens)
- `.compact` → smaller buttons for inline actions (delete, toggle, remove)
- `.visually-hidden` → accessible hidden labels
- `.inline` → forms embedded in table cells/toolbars without extra margin

All examples will link this via `<link rel="stylesheet" href="/livetemplate.css">` and the Go server will serve it.

---

## Phase 2: Universal HTML & Visual Fixes (All Templates)

Apply to ALL 10 templates:

1. **Add `lang="en"`** to `<html>` where missing (6 templates)
2. **Add `<meta name="color-scheme" content="light dark">`** to all `<head>` blocks
3. **Remove `data-theme="light"`** from todos template
4. **Link shared CSS**: `<link rel="stylesheet" href="/livetemplate.css">` in all templates
5. **Serve shared CSS**: Add `http.Handle("/livetemplate.css", ...)` in each example's main.go (or via the testing framework)
6. **Standardize `<title>`**: Use `App Name — LiveTemplate` pattern
7. **Use `<hgroup>`** for title + subtitle combinations consistently

---

## Phase 3: Per-Example Visual & Code Fixes

### 3a. `counter/counter.tmpl` — Visual Overhaul
**Current**: Counter value is plain `<p>Counter: X</p>`, 3 full-width buttons in grid.
**Improved**:
- Wrap in `<article>` with `<hgroup>` (title + "Real-time counter with WebSocket" subtitle)
- Make counter value visually prominent: `<p><strong>Counter: <data>{{.Counter}}</data></strong></p>`
- Use `<fieldset role="group">` for buttons instead of `<div class="grid">` — renders as a compact single-row toolbar
- Add `lang="en"`, `color-scheme` meta

### 3b. `todos/todos.tmpl` — Compact Controls, Tighter Layout
**Current**: Add form → stats → search → sort dropdown → table → pagination → clear completed. Very vertical.
**Improved**:
- Remove `<style>` block, link shared CSS
- Remove `data-theme="light"`
- **Combine search + sort into one row**: Use `<div class="grid">` with search input + sort select side by side
- **Stats as subtitle**: Move stats into `<hgroup>` as `<p>` subtitle, not a separate section
- **Compact delete buttons**: Add `class="compact secondary"` to delete buttons in table
- **Compact pagination**: Smaller previous/next buttons with `class="compact"`
- **Clear completed**: Make it a `class="compact secondary outline"` button, not full-width

### 3c. `chat/chat.tmpl` — Reduce CSS, Compact Input
**Current**: ~50 lines custom CSS, full-width send button separate from input.
**Improved**:
- Replace `.stats` div → `<small>` element (Pico handles the styling)
- Replace `.input-form` grid → `<fieldset role="group">` (compact input+button on one line)
- Replace `.empty-state` → `<small>` 
- Keep message container `.messages` CSS (genuine Tier 2 need for scrollable area with `lvt-scroll`)
- Keep `.message` styles but minimize — use Pico variables
- Remove `body { padding: 1rem; }` — Pico's container handles this
- Wrap in `<article>` for consistent card structure
- Add `<hgroup>` with title + online status

### 3d. `flash-messages/flash.tmpl` — Single Toolbar
**Current**: "Clear All" and "Simulate Error" as two full-width buttons in a grid, full-width "Add Item" button.
**Improved**:
- **Combine add form**: Use `<fieldset role="group">` for input + "Add" button on one line
- **Action toolbar**: Use `<nav>` or `<fieldset role="group">` for "Clear All" + "Simulate Error" as compact buttons in one row
- **Compact remove buttons**: `class="compact secondary outline"` in item table
- Add `<hgroup>` with title + subtitle
- Add `lang="en"`, `color-scheme` meta
- Move the info blockquote to a `<details>` accordion (saves vertical space — user can expand if curious)

### 3e. `progressive-enhancement/progressive-enhancement.tmpl` — Table Layout, No Custom CSS
**Current**: Custom `<style>` block with `.completed`, `.empty-state`, `.todo-item`, `.js-mode`. Each item is a flex row.
**Improved**:
- **Remove entire `<style>` block**
- **Table layout**: Convert todo items to `<table>` rows: checkbox | title | date | delete
- `.completed` class → `<s>` element (per CLAUDE.md)
- `.empty-state` → `<p><small>...</small></p>`
- **Compact toggle/delete buttons**: `class="compact secondary outline"` / `class="compact contrast outline"`
- **Remove JS mode indicator** script — keep only `<noscript>` block
- Use `<fieldset role="group">` for add form (input + button on one line)
- Add `<hgroup>` with title + subtitle

### 3f. `ws-disabled/ws-disabled.tmpl` — Table Layout, No Inline Styles
**Current**: 5 inline `style` attributes, flex layout for bookmarks.
**Improved**:
- **Replace ALL inline styles** with Pico semantic HTML
- **Table layout**: Convert bookmarks to `<table>` rows: link + URL | delete
- **Compact delete buttons**: `class="compact contrast outline"`
- Empty state: `<p><small>No bookmarks yet. Add one above!</small></p>`
- Use `<fieldset role="group">` for add form (label input + URL input on one row? or keep grid but tighter)
- Add `<hgroup>` with title + subtitle
- Add `lang="en"`, `color-scheme` meta

### 3g. `shared-notepad/notepad.tmpl` — Remove Inline JS
**Current**: `oninput="..."` for char count, footer inside `<form>`.
**Improved**:
- **Remove inline JS**: Add `Change()` method to Go controller for reactive char count
- **Restructure**: Move save button into a `<fieldset role="group">` below textarea (button right-aligned, compact)
- Char count as `<small>` below textarea, updated server-side
- Add `lang="en"`, `color-scheme` meta

### 3h. `avatar-upload/avatar-upload.tmpl` — Remove Custom JS, Clean Layout
**Current**: Custom JS event listeners, outdated footer.
**Improved**:
- **Remove custom JavaScript** event listeners (framework handles upload events natively)
- **Fix footer**: Remove outdated "LiveTemplate v0.3.0" reference
- **Compact save button**: Profile forms don't need a full-width save button — use `class="compact"` or keep full-width (it's the primary action, so full-width is OK)
- Avatar display: Keep as-is (simple img/mark pattern works)
- Add `color-scheme` meta

### 3i. `live-preview/preview.tmpl` — Minor Polish
**Current**: Clean and simple. Missing `lang="en"`.
**Improved**:
- Add `lang="en"`, `color-scheme` meta
- Add `<hgroup>` with subtitle "Type to see a live preview"
- Link shared CSS

### 3j. `login/templates/auth.html` — Minor Polish
**Current**: Already well-structured with proper form patterns.
**Improved**:
- Add `color-scheme` meta
- Link shared CSS (for narrow container benefit)
- Compact logout button: `class="compact contrast"` (it's a secondary action)

---

## Phase 4: Go Controller Changes

### 4a. `shared-notepad/main.go`
- Add `Change()` method to handle real-time textarea input
- Calculate char count server-side: `state.CharCount = len([]rune(state.Content))`
- The Change() method enables auto-wired input bindings — textarea changes trigger server-side char count update

### 4b. All examples — serve shared CSS
- Each example's main.go serves `/livetemplate.css` at runtime via `http.ServeFile` pointing to `../client/livetemplate.css`
- Templates link it with `<link rel="stylesheet" href="/livetemplate.css">`
- For production/CDN, the CSS would be published alongside the client JS package

---

## Phase 5: Update CLAUDE.md Guidelines

Add new sections:

### Shared CSS
```
- ALL examples link `livetemplate.css` for shared utilities
- Served at `/livetemplate.css` (dev) or CDN (production)
- Contains: narrow container, .compact buttons, .visually-hidden, .inline forms
```

### HTML Boilerplate Standard
```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <title>App Name — LiveTemplate</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
    <link rel="stylesheet" href="/livetemplate.css">
</head>
```

### Visual Layout Standard
- Wrap content in `<main class="container">` (narrow 640px via shared CSS)
- Use `<article>` as the primary card container
- Use `<hgroup>` for title + subtitle: `<hgroup><h1>Title</h1><p>Subtitle</p></hgroup>`
- Group related controls in single-row toolbars: `<fieldset role="group">` or `<nav>`

### Button Sizing
- Primary actions: full-width or auto-width Pico buttons
- Inline/table actions (delete, toggle, remove): `class="compact"` + `secondary outline` or `contrast outline`
- Forms inside tables/toolbars: `class="inline"` to remove margin

### Form Pattern Standard
- Use `<fieldset role="group">` for inline input+button groups (e.g., add forms, search bars)
- Place `{{.lvt.ErrorTag}}` immediately after the input it validates
- Consistent empty states: `<p><small>Message here</small></p>`

### List/Item Display Standard
- Use `<table>` for item lists with actions (consistent column layout)
- Use `<s>` for completed/struck-through items
- Use `hidden` attribute for conditional visibility

---

## Phase 6: Verify Tests

Run `./test-all.sh` to ensure all examples still pass. Test adjustments likely needed for:
- Element selectors changed (e.g., `div.todo-item` → `tr`, `div.grid button` → `fieldset[role=group] button`)
- Text content changes (e.g., titles with " — LiveTemplate" suffix)
- New DOM structure affecting chromedp queries
- Removed JS mode indicator in progressive-enhancement

---

## Todos Summary

| ID | Title | Dependencies |
|---|---|---|
| `shared-css` | Create livetemplate.css with visual utilities | — |
| `serve-css` | Add CSS serving to all examples' main.go | `shared-css` |
| `fix-counter` | Counter: fieldset button group, hgroup, HTML fixes | `serve-css` |
| `fix-todos` | Todos: compact controls, combined search+sort, remove custom CSS | `serve-css` |
| `fix-chat` | Chat: reduce CSS, fieldset input, article wrapper | `serve-css` |
| `fix-flash` | Flash: single toolbar, compact buttons, details accordion | `serve-css` |
| `fix-progressive` | Progressive: table layout, remove all custom CSS/JS | `serve-css` |
| `fix-ws-disabled` | WS-disabled: table layout, remove all inline styles | `serve-css` |
| `fix-notepad` | Notepad: remove inline JS, add Change() method | `serve-css` |
| `fix-avatar` | Avatar: remove custom JS, fix footer | `serve-css` |
| `fix-live-preview` | Live-preview: hgroup, HTML fixes | `serve-css` |
| `fix-login` | Login: compact logout, color-scheme | `serve-css` |
| `update-claude-md` | Add visual + coding guidelines to CLAUDE.md | all fixes |
| `verify-tests` | Run test-all.sh and fix any test breakage | all fixes |
