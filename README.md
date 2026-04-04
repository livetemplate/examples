# LiveTemplate Examples

Example applications demonstrating LiveTemplate usage with various features and patterns.

## Progressive Complexity

All examples follow the [progressive complexity](https://github.com/livetemplate/livetemplate/blob/main/docs/guides/progressive-complexity.md) model. Tier 1 (standard HTML) is preferred; Tier 2 (`lvt-*` attributes) is used only when necessary.

| Example | Tier | Description | Tier 2 Attributes |
|---------|------|-------------|--------------------|
| `counter/` | 1 | Counter with logging + graceful shutdown | None |
| `chat/` | 1+2 | Real-time multi-user chat | `lvt-fx:scroll` |
| `todos/` | 1+2 | Full CRUD with SQLite, auth, modal + toast components | Component-internal |
| `flash-messages/` | 1 | Flash notification patterns | None |
| `avatar-upload/` | 1+2 | File upload with progress | `lvt-upload` |
| `progressive-enhancement/` | 1 | Works with/without JS | None |
| `ws-disabled/` | 1 | HTTP-only mode | None |
| `live-preview/` | 1 | Change() live updates | None |
| `login/` | 1+2 | Authentication + sessions | `lvt-form:no-intercept` |
| `shared-notepad/` | 1+2 | BasicAuth + SharedState | `lvt-form:preserve` |

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
<script src="https://cdn.jsdelivr.net/npm/@livetemplate/client@0.1.0/dist/livetemplate-client.browser.js"></script>
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
