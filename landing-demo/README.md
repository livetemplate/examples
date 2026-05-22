# landing-demo

The minimal LiveTemplate counter that powers the live demo on
[livetemplate.fly.dev](https://livetemplate.fly.dev). Deployed standalone
as `lt-landing-demo.fly.dev` and proxied same-origin by the docs site so
the landing page can iframe it without cross-origin friction.

The whole app is `main.go` (~50 lines) plus `counter.tmpl` (~25 lines).
Same code, three transports:

- **Without JS**: form POST, page reloads with new state.
- **With the JS client (fetch)**: same form POSTs via `fetch()`; the DOM is patched in place.
- **With WebSocket**: actions ride the WS; other tabs in the same browser session sync automatically.

## How cross-tab sync works

Two pieces enable it:

- `Count int \`lvt:"persist"\`` makes the field session-store backed, so it survives reconnects and is visible to every connection in the same session group.
- `Mount` opts this connection into peer fan-out via `ctx.Subscribe(ctx.SelfTopic())`, and each counter handler ends with `ctx.Publish(ctx.SelfTopic(), "Increment", nil)` so peer tabs receive the same counter action and re-render.

Without the `Subscribe`/`Publish` pair, peer tabs would only see the latest value on their own next action or a full reload. Without `persist`, a full reload would not preserve the session count.

## Run locally

```bash
go run .
```

Then open http://localhost:8080.

## Deploy

```bash
flyctl deploy --remote-only
```
