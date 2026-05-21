# LiveTemplate Examples

Example applications demonstrating LiveTemplate usage with various features and patterns.

📚 **Framework documentation:** **<https://livetemplate.fly.dev>** — guides, recipes, patterns catalog (with live demos), full reference. The `/examples` section of the docs site indexes every app in this repo. The `/patterns` catalog is served from the docs repo's [`content/recipes/patterns/_app/`](https://github.com/livetemplate/docs/tree/main/content/recipes/patterns/_app/) package.

## Showcase: Todo App

The todo app demonstrates LiveTemplate's core features in ~150 lines of Go + ~80 lines of HTML:

- **Real-time sync** — open two tabs as the same user; changes appear instantly via opt-in `ctx.Subscribe` / `ctx.Publish`
- **Standard HTML forms** — `<form method="POST" name="add">` routes to `Add()` with zero configuration
- **Live search & sort** — `Change()` auto-wires input events with 300ms debounce
- **Validation** — `ErrorTag`, `AriaInvalid`, `AriaDisabled` template helpers
- **Components** — modal confirmation dialogs and toast notifications
- **Entry animations** — `lvt-fx:animate="fade"` on new rows
- **Loading states** — `lvt-el:setAttr:on:add:pending="aria-busy:true"` for visual feedback
- **Dark mode** — automatic via `<meta name="color-scheme" content="light dark">`
- **Progressive enhancement** — standard form actions use HTTP POST fallback; live search/sort and other reactive interactions are enhanced by JavaScript

## Progressive Complexity

All examples follow the [progressive complexity](https://github.com/livetemplate/livetemplate/blob/main/docs/guides/progressive-complexity.md) model. Tier 1 (standard HTML) is preferred; Tier 2 (`lvt-*` attributes) is used only when necessary.

| Example | Tier | Description | Tier 2 Attributes |
|---------|------|-------------|--------------------|
| `counter/` | 1 | Counter with logging + graceful shutdown | None |
| `chat/` | 1+2 | Real-time multi-user chat | `lvt-fx:scroll` |
| `todos/` | 1+2 | Full CRUD with SQLite, auth, modal + toast components | `lvt-on:change`, `lvt-fx:animate`, `lvt-fx:highlight`, `lvt-el:setAttr` |
| `flash-messages/` | 1 | Flash notification patterns | None |
| `avatar-upload/` | 1+2 | File upload with progress | `lvt-upload` |
| `progressive-enhancement/` | 1 | Works with/without JS | None |
| `ws-disabled/` | 1 | HTTP-only mode | None |
| `live-preview/` | 1 | Change() live updates | None |
| `login/` | 1+2 | Authentication + sessions | `lvt-form:no-intercept` |
| `shared-notepad/` | 1+2 | BasicAuth + SharedState | `lvt-form:preserve` |
| `dialog-patterns/` | 1 | Native `<dialog>` with `command`/`commandfor` | None (polyfilled by client) |

## Examples

The directories listed in the table above are individual example applications. Each folder contains a minimal, self-contained project that demonstrates a specific LiveTemplate pattern or feature.

## Running Examples

Each example is self-contained with its own `go.mod`. To run an example:

```bash
cd <example-directory>
go mod download
go run main.go
```

## Testing Examples

### Test All Examples

Run all working examples at once:

```bash
./test-all.sh
```

This script will:
- Test all 5 working examples (counter, chat, todos, graceful-shutdown, testing)
- Skip disabled examples by default (use without `--skip-disabled` to attempt them)
- Show a summary of passed/failed/skipped tests

### Test Individual Example

Examples include E2E tests using Chromedp:

```bash
cd <example-directory>
go test -v
```

### CI/CD

The test script is also used in GitHub Actions. See `.github/workflows/test.yml` for CI configuration.

## Using the Client Library

### Production (CDN)

Examples are configured to use the CDN version of the client library:

```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/livetemplate.css">
<script defer src="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/dist/livetemplate-client.browser.js"></script>
```

### Development (Local)

For local development, examples can serve the client library locally using `github.com/livetemplate/lvt/testing`.

## Dependencies

- **Core Library**: `github.com/livetemplate/livetemplate v0.8.15`
- **LVT Testing** (for examples with E2E tests): `github.com/livetemplate/lvt` (latest)
- **Client Library**: `@livetemplate/client@latest` (via CDN)

## Related Projects

- **[LiveTemplate Core](https://github.com/livetemplate/livetemplate)** - Go library for server-side rendering
- **[Client Library](https://github.com/livetemplate/client)** - TypeScript client for browsers
- **[LVT CLI](https://github.com/livetemplate/lvt)** - Code generator and development server

## Version Synchronization

Examples follow the LiveTemplate core library's major.minor version:
- Core: `v0.1.5` → Examples: `v0.1.x` (any patch version)
- Core: `v0.2.0` → Examples: `v0.2.0` (must match major.minor)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on adding new examples.

## License

MIT License - see [LICENSE](LICENSE) for details.
