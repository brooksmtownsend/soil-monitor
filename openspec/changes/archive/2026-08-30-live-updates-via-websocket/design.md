## Context

The web display is a single-binary, no-external-asset HTTP server in `internal/webdisplay`. Read-only pages currently embed `<meta http-equiv="refresh" content="5">`, which triggers a full-page reload every five seconds and causes a visible flash. See `proposal.md — Why` for motivation.

The application's main event loop lives in `internal/app.Runner.Run`. It already calls `r.Display.Render()` on every state change. The `core.WebServer` interface is currently `Run(ctx) error` only. The `gorilla/websocket` package is already a transitive dependency (via `paho.mqtt.golang`) and can be promoted to direct without a new module fetch.

## Goals / Non-Goals

**Goals:**
- Eliminate the full-page flash by pushing state updates over a persistent WebSocket connection.
- Keep the implementation entirely within the standard library plus the already-available `gorilla/websocket`.
- Notify connected WebSocket clients from the same event loop that already drives OLED re-renders, preserving the single-coordinator invariant.
- Provide a graceful fallback: the server-rendered HTML is still correct on initial load and on reconnect; JS is only for live patching.

**Non-Goals:**
- Partial DOM diffing or a virtual DOM — a full snapshot push per event is sufficient given the small payload size.
- WebSocket authentication or TLS (trusted-LAN service; same as the rest of the HTTP display).
- Supporting the plant detail chart via WebSocket (the chart is history-backed and only changes on reconnect/reload, not on live readings; live data there is future work).

## Decisions

### 1. Notification path: extend `core.WebServer` with `Notify(snapshot)`

**Decision**: Add `Notify(core.Snapshot)` to the `core.WebServer` interface and call it from `Runner.Run` wherever `redraw` is true, after any MQTT, button, or rename event. The `webdisplay.Server` implements it by forwarding to the hub.

**Alternatives considered**:
- *Separate channel passed into Server*: Works but adds wiring boilerplate; the interface extension is cleaner and keeps the Runner's web interaction symmetric with how it calls `Display.Render()`.
- *Server polls State on a ticker*: Avoids interface changes but re-introduces polling lag and is strictly worse.
- *Broadcast channel on Runner*: Possible but exposes internals; the interface approach keeps `Runner` decoupled from `webdisplay`.

### 2. Hub goroutine in `webdisplay.Server`

**Decision**: The `Server` starts an internal hub goroutine when `Run` is called. The hub owns a `map[*client]struct{}` of live WebSocket connections and reads from a single unbuffered `notify chan core.Snapshot` channel. `Notify()` sends to that channel (non-blocking: if the hub is busy it skips — stale updates are not harmful given the push-on-event model).

**Alternatives considered**:
- *Mutex-guarded client map with direct write in Notify*: Avoids an extra goroutine but requires fine-grained locking across the write path and the client registration path; the hub goroutine is simpler to reason about.

### 3. Client goroutines per WebSocket connection

**Decision**: Each accepted WebSocket connection gets one goroutine for reading (ping/pong/close frames) and writes are driven by the hub. The hub serializes writes to each client and removes clients that fail a write.

### 4. JSON snapshot payload

**Decision**: The server sends a JSON object derived from `core.Snapshot` — including `connected`, plant IDs, display names, latest percentages, raw values, and observed-at timestamps. The client patches the DOM using `data-plant-id` attributes rather than a full innerHTML replace to avoid layout flicker.

**Alternatives considered**:
- *HTML fragment push (htmx-style)*: Cleaner server side, but adds complexity for matching elements and still requires inline JS; JSON is simpler given the small payload.
- *Server-Sent Events (SSE)*: Would avoid the need for `gorilla/websocket`, but SSE is unidirectional and would require an additional mechanism if future bi-directional needs arise. WebSocket is the user's stated preference and consistent with the intent.

### 5. Promote `gorilla/websocket` to direct dependency

**Decision**: Run `go get github.com/gorilla/websocket` to promote it from indirect to direct. The package is already resolved and vendored; this is a `go.mod` annotation change only.

## Risks / Trade-offs

- **Inline JS maintenance**: The inline script block is small (~40 lines) but lives in a Go string constant. Any bugs require a Go recompile. [Risk] → Keep the script minimal and covered by a browser-agnostic integration test or manual smoke test.
- **Hub goroutine leak on test teardown**: If `Run` is not called (test constructs Server directly), the hub is not started. [Risk] → Hub is started only inside `Run`; tests that need broadcast behavior must start the server or inject a fake.
- **Skipped notifications on busy hub**: Non-blocking send means a burst of rapid changes could skip intermediate states. [Risk] → Acceptable; the client always receives the *next* push and intermediate values are not guaranteed by the spec.

## Migration Plan

1. Promote `gorilla/websocket` to direct dependency (`go get`).
2. Add `Notify(core.Snapshot)` to `core.WebServer` interface and implement a no-op stub for tests.
3. Add hub, client types, and `/ws` handler to `internal/webdisplay`.
4. Remove `<meta http-equiv="refresh">` from both HTML templates and add inline WebSocket `<script>` block.
5. Update `Runner.Run` to call `r.Web.Notify(r.State.Snapshot())` after each `redraw` event.
6. Update `internal/webdisplay/server_test.go` with WebSocket endpoint tests.
7. Run `mise run test` and `mise run vet`; smoke-test on the Pi dashboard.

Rollback: revert the `<meta http-equiv="refresh">` tags and remove the `/ws` route; no schema or data migration required.
