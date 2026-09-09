## 1. Newest-First Table Rendering

- [x] 1.1 Derive a reversed copy of each detail page's chronological reading slice for table rendering without mutating the slice used by chart generation; verify focused helper or handler tests cover empty, single-reading, descending timestamp, and equal-timestamp reverse receipt order with `go test ./internal/webdisplay`.
- [x] 1.2 Render table rows from the reversed copy and update the accessible caption to describe newest-to-oldest order; verify handler HTML assertions show the newest raw value before the oldest while existing chart-point assertions still prove oldest-to-newest chart order with `go test ./internal/webdisplay`.

## 2. Verification and Specification Completion

- [x] 2.1 Run `mise run fmt`, `mise run test`, and `mise run vet`, fixing any regressions and verifying the complete supported suite and static analysis pass.
- [x] 2.2 Run `mise exec -- openspec validate sort-history-table-newest-first --strict` and sync the newest-first table behavior into the `local-web-display` main specification without changing storage-query ordering.

## 3. Eastern Time Presentation

- [x] 3.1 Add a shared DST-aware `America/New_York` conversion backed by the embedded standard-library time-zone database, and use it for overview observations, history rows, and range-boundary display text while retaining RFC 3339 machine instants; verify summer `EDT`, winter `EST`, and unchanged instants with `go test ./internal/webdisplay`.
- [x] 3.2 Apply the shared Eastern conversion to WebSocket `observedAtDisplay` values and verify a live update matches the corresponding server-rendered Eastern timestamp with `go test ./internal/webdisplay`.
- [x] 3.3 Update dashboard documentation to distinguish Eastern human-readable display from UTC persistence and range filtering, and verify the documented behavior matches the HTML and WebSocket tests.
- [x] 3.4 Run `mise run fmt`, `mise run test`, `mise run vet`, and strict OpenSpec validation; sync the revised `local-web-display` delta and verify every implementation task is checked before archive.
