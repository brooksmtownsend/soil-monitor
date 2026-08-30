## 1. Dependency and Interface

- [x] 1.1 Promote `gorilla/websocket` from indirect to direct in `go.mod` by running `go get github.com/gorilla/websocket` and verify `go.mod` lists it without the `// indirect` comment.
- [x] 1.2 Add `Notify(core.Snapshot)` to the `core.WebServer` interface in `internal/core/` and verify `go build ./...` passes (the `webdisplay.Server` will satisfy it once step 2 is done).

## 2. WebSocket Hub and Handler

- [x] 2.1 Add `hub`, `client`, and related types to `internal/webdisplay/` with a `notify chan core.Snapshot` field on `Server`; start the hub goroutine inside `Server.Run` and verify via `go vet ./internal/webdisplay/...`.
- [x] 2.2 Implement `Server.Notify(core.Snapshot)` as a non-blocking send to the hub channel and verify `go build ./...` passes and the `core.WebServer` interface is satisfied.
- [x] 2.3 Add the `/ws` HTTP route to `Server.newHandler` that upgrades to WebSocket, registers the client with the hub, and pushes the current snapshot immediately on connect; verify the route is reachable and returns `101 Switching Protocols` in a test.

## 3. HTML Template Updates

- [x] 3.1 Remove `<meta http-equiv="refresh" content="5">` from the `overviewHTML` template constant and verify the constant no longer contains the tag.
- [x] 3.2 Remove `<meta http-equiv="refresh" content="5">` from the `detailHTML` template constant and verify the constant no longer contains the tag.
- [x] 3.3 Add an inline `<script>` block to both templates that opens `ws://<host>/ws`, receives JSON snapshots, and updates `data-plant-id`-keyed DOM elements in place; include exponential-backoff reconnect logic; verify `go build ./...` passes and the script is present in rendered HTML in a test.

## 4. Runner Integration

- [x] 4.1 Update `Runner.Run` in `internal/app/app.go` to call `r.Web.Notify(r.State.Snapshot())` alongside any `r.Display.Render()` call after each `redraw` event (MQTT, button, rename); verify `go build ./...` passes.
- [x] 4.2 Verify fake/stub `WebServer` implementations used in `internal/app/app_test.go` satisfy the updated `core.WebServer` interface (add a no-op `Notify` method to each stub) and that `go test ./internal/app/...` passes.

## 5. Tests

- [x] 5.1 Add a test in `internal/webdisplay/server_test.go` that dials the `/ws` endpoint, verifies the server sends an initial snapshot JSON immediately on connect, and that a subsequent `Notify` call pushes an updated snapshot to the connected client; verify the test passes with `go test ./internal/webdisplay/...`.
- [x] 5.2 Add a test that verifies a client removed from the hub (closed connection) does not cause a panic or goroutine leak on subsequent `Notify` calls.

## 6. Validation

- [x] 6.1 Run `mise run vet` and confirm zero diagnostics.
- [x] 6.2 Run `mise run test` and confirm all tests pass.
- [x] 6.3 Run `openspec validate --change live-updates-via-websocket` and confirm no errors.
