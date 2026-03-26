# Profile Editor — Progressive Complexity (Tier 1)

A profile editing form built with **zero `lvt-*` attributes**. The form auto-submits to the conventional `Submit()` method.

This demonstrates LiveTemplate's [progressive complexity model](https://github.com/livetemplate/livetemplate/blob/main/docs/guides/progressive-complexity.md) — the simplest possible form.

## What It Shows

- **Auto-submit**: `<form method="POST">` with no `lvt-submit` routes to `Submit()`
- **Server-side validation**: Uses `BindAndValidate()` with struct tags
- **Error display**: `.lvt.HasError` and `.lvt.Error` template helpers
- **Preview section**: Current state rendered alongside the form
- **Progressive enhancement**: Works with and without JavaScript

## Running

```bash
cd profile-progressive
go run main.go
```

Open http://localhost:8081

## Key Pattern

The entire form uses zero custom attributes:

```html
<form method="POST">
    <input type="text" name="DisplayName" value="{{.DisplayName}}">
    <input type="email" name="Email" value="{{.Email}}">
    <textarea name="Bio">{{.Bio}}</textarea>
    <button type="submit">Save Profile</button>
</form>
```

The framework auto-intercepts the form and routes it to `Submit()` — the conventional default when no button name or form name is specified.

## Next Steps

- Add `<button name="save-draft" formnovalidate>` for a draft save button
- Add a `Change()` method for live preview updates (Phase 2: inferred bindings)
- See the [todos-progressive](../todos-progressive/) example for multi-action routing
