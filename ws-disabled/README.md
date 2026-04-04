# WebSocket Disabled Example

This example demonstrates LiveTemplate's `WithWebSocketDisabled()` mode. The client library is still included and handles all interactions — but uses HTTP fetch instead of WebSocket to send actions and receive tree-based DOM updates.

## When to Use This Mode

- **Constrained environments**: Firewalls, proxies, or platforms that block WebSocket connections
- **Simpler deployment**: No need for WebSocket-aware load balancers or reverse proxies
- **API backends**: Serve tree-based JSON updates to htmx, Alpine.js, or custom JS clients

## How It Works

### Client Library Still Active

The LiveTemplate client library is included and works the same as in WebSocket mode:
- Standard HTML forms with `<button name="action">` and `<form name="action">` work identically
- DOM updates are applied via tree diffing — no page reloads
- The only difference is transport: HTTP fetch instead of WebSocket

### Detection

The client detects WebSocket availability by checking the `X-LiveTemplate-WebSocket` response header:
- `enabled` — client connects via WebSocket (default)
- `disabled` — client falls back to HTTP fetch

### Progressive Enhancement

Forms use `method="POST"` with button `name` routing, so the app still works without JavaScript via the POST-Redirect-GET pattern.

## Running

```bash
# Development mode (serves client library locally)
LVT_DEV_MODE=true go run .
```

Visit http://localhost:8080.

## Files

- `main.go` — Controller, state, and action handlers
- `ws-disabled.tmpl` — Template with standard HTML forms and client library
- `ws_disabled_test.go` — Browser and HTTP e2e tests
- `README.md` — This documentation
