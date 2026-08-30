## Why

The dashboard refreshes by issuing a full-page reload every 5 seconds via `<meta http-equiv="refresh">`, which causes a visible screen flash on every cycle. Replacing this with a WebSocket-push approach lets the server send updated state the moment it changes and the browser update the DOM in place, eliminating the flash entirely.

## What Changes

- Remove the `<meta http-equiv="refresh">` tag from the overview and detail HTML templates.
- Add a WebSocket endpoint (`GET /ws`) to the HTTP server that pushes JSON state snapshots to connected browsers whenever readings change.
- Add a small inline `<script>` block to each page that opens the WebSocket, receives snapshots, and updates the relevant DOM elements without a full reload.
- The WebSocket connection also sends a snapshot on connection establishment so the page is immediately current.
- When the WebSocket is closed or errors (e.g. server restart), the client falls back to a periodic polling reconnect so it self-heals without user action.

## Capabilities

### New Capabilities

_(none — this change modifies an existing capability only)_

### Modified Capabilities

- `local-web-display`: The "Periodic static refresh" requirement is replaced. Read-only pages SHALL use a server-pushed WebSocket connection to receive live state updates instead of a full-page meta-refresh. Inline JavaScript is now permitted to manage the WebSocket and apply DOM patches. The no-external-assets and no-external-JavaScript-library constraints remain.

## Impact

- `internal/webdisplay/server.go` — new `/ws` route, WebSocket upgrade, broadcast logic.
- `internal/webdisplay/server.go` HTML templates — remove `<meta http-equiv="refresh">`, add inline WebSocket `<script>` block.
- `internal/webdisplay/server_test.go` — tests for the `/ws` endpoint and snapshot broadcast behavior.
- No new third-party Go dependencies; the standard library `golang.org/x/net/websocket` or a minimal upgrade handler can be used. Given the no-external-deps preference, the implementation will use `net/http` upgrade headers directly or the `golang.org/x/net/websocket` package already available in the Go standard distribution.
- The `local-web-display` spec's "Periodic static refresh" requirement is replaced; all other requirements are unchanged.
