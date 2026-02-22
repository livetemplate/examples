# LiveTemplate Examples

Example applications demonstrating LiveTemplate usage with various features and patterns.

## Examples

### 1. Counter - Simple State Management
**Directory**: `counter/`

Basic counter demonstrating reactive state updates.

```bash
cd counter
go run main.go
# Visit http://localhost:8080
```

**Features**:
- Simple state management
- Button click handling
- Real-time updates

---

### 2. Chat - Real-time Communication
**Directory**: `chat/`

Multi-user chat application with WebSocket communication.

```bash
cd chat
go run main.go
# Visit http://localhost:8080
```

**Features**:
- Multi-user chat rooms
- WebSocket messaging
- User presence
- Message history

---

### 3. Todos - Full CRUD Application
**Directory**: `todos/`

Complete todo list with database, validation, and full CRUD operations.

```bash
cd todos
go run main.go
# Visit http://localhost:8080
```

**Features**:
- SQLite database
- CRUD operations
- Form validation
- Database migrations
- E2E tests with Chromedp

---

### 4. Graceful Shutdown
**Directory**: `graceful-shutdown/`

Demonstrates proper server shutdown handling.

```bash
cd graceful-shutdown
go run main.go
# Press Ctrl+C to trigger graceful shutdown
```

**Features**:
- Signal handling
- Connection draining
- Cleanup procedures

---

### 5. Observability
**Directory**: `observability/`

Logging, metrics, and tracing example.

```bash
cd observability
go run main.go
```

**Features**:
- Structured logging
- Custom metrics
- Request tracing
- Performance monitoring

---

### 6. Testing
**Directory**: `testing/01_basic/`

E2E testing patterns with Chromedp.

```bash
cd testing/01_basic
go test -v
```

**Features**:
- Browser automation
- E2E test patterns
- Test helpers
- Assertions

---

### 7. Production
**Directory**: `production/single-host/`

Production deployment configuration.

```bash
cd production/single-host
go run main.go
```

**Features**:
- Production server setup
- Environment configuration
- Health checks
- Deployment best practices

---

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

- **Core Library**: `github.com/livetemplate/livetemplate v0.1.0`
- **LVT Testing** (for examples with E2E tests): `github.com/livetemplate/lvt v0.1.0`
- **Client Library**: `@livetemplate/client@0.1.0` (via CDN)

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
