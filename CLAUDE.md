# Examples Conventions

## Progressive Complexity Pattern

All examples MUST follow the progressive complexity pattern introduced in LiveTemplate v0.8.6:

### Tier 1 (Standard HTML) — Use by default
- **Form submission:** `<form method="POST">` with `<button type="submit" name="ActionName">` (e.g., `name="add"` routes to `Add()`)
- **Multiple actions:** Use distinct button `name` attributes for routing (e.g., `name="toggle"` routes to `Toggle()`, `name="delete"` routes to `Delete()`)
- **Data passing:** `<input type="hidden" name="id" value="{{.ID}}">` (NOT `lvt-data-*` attributes)
- **Live input binding:** Add a `Change()` method to the controller + `value="{{.Field}}"` on inputs
- **Validation:** Use HTML attributes (`required`, `type="email"`, `minlength`, etc.)
- **Dialogs:** Native `<dialog>` with `command="show-modal"` / `command="close"`
- **Loading states:** Automatic `aria-busy="true"` on forms + `<fieldset>` disabled during submission

### Tier 2 (`lvt-*` attributes) — Only when Tier 1 is insufficient
- `lvt-scroll` — auto-scroll behavior (no HTML equivalent)
- `lvt-upload` — file upload with progress tracking
- `lvt-disable-with` — button loading text
- `lvt-change` on `<select>` — auto-submit on select change (no Tier 1 auto-submit for selects)
- `lvt-change` on `<input type="checkbox">` — auto-submit on checkbox change to a specific action
- `lvt-debounce` / `lvt-throttle` — custom rate limiting (default 300ms via Change() is Tier 1)
- `lvt-key` — keyboard shortcuts
- `lvt-click-away` — click outside detection
- Component templates (`lvt:modal:*`, `lvt:toast:*`)

### Pattern Reference
- Button name routing: `<button name="X">` routes to `X()` Go method
- Form default action: `<form method="POST">` without named button routes to `Submit()`
- Change() auto-binding: inputs with `value="{{.Field}}"` auto-bind when controller has `Change()` method (300ms debounce)

### Docs
- Reference: https://github.com/livetemplate/livetemplate/blob/main/docs/references/progressive-complexity-reference.md
- Guide: https://github.com/livetemplate/livetemplate/blob/main/docs/guides/progressive-complexity.md
