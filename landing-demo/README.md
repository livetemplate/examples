# landing-demo

The minimal LiveTemplate counter that powers the live demo on
[livetemplate.fly.dev](https://livetemplate.fly.dev). Deployed standalone
as `lt-landing-demo.fly.dev` and proxied same-origin by the docs site so
the landing page can iframe it without cross-origin friction.

The whole app is `main.go` (~50 lines) plus `counter.tmpl` (~30 lines).
Same code, three transports:

- **Without JS**: form POST, page reloads with new state.
- **With the JS client (fetch)**: same form POSTs via `fetch()`; the DOM is patched in place.
- **With WebSocket**: actions ride the WS; other tabs in the same session sync automatically.

Per-session ephemeral state (no DB). Each visitor has their own counter;
their own tabs stay in sync via WebSocket.

## Run locally

```bash
go run .
```

Then open http://localhost:8080.

## Deploy

```bash
flyctl deploy --remote-only
```
