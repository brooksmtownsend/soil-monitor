## 1. Time-Bounded History Storage

- [x] 1.1 Extend the history repository contract and SQLite query to accept inclusive UTC start/end instants, apply both bounds before the newest-reading limit, and preserve deterministic chronological output; verify with `go test ./internal/history`.
- [x] 1.2 Add storage tests for lower/upper boundary inclusion, exclusion outside the interval, newest-1,000 selection within the interval, empty known-plant results, unknown plants, and invalid interval/limit inputs; verify with `go test ./internal/history`.
- [x] 1.3 Update repository fakes and callers for the interval-aware contract without changing ingestion or restoration behavior; verify with `go test ./internal/app ./internal/webdisplay`.

## 2. Range Selection and Rendering

- [x] 2.1 Add the six preset definitions, fixed request clock, UTC boundary calculation, and end-of-month clamping; verify table-driven tests cover every token plus month-end and leap-year cases with `go test ./internal/webdisplay`.
- [x] 2.2 Parse the detail page's `range` query with a missing-value default of `1d`, reject empty or unsupported values before history lookup, and pass the selected closed interval to storage; verify handler tests cover default, each supported value, HTTP 400, HTTP 404, and interval arguments with `go test ./internal/webdisplay`.
- [x] 2.3 Render canonical links for all six presets, mark the active preset, describe the selected interval and 1,000-row cap, and distinguish an empty selected range from absent plant data; verify HTML assertions cover populated, capped, and empty ranges with `go test ./internal/webdisplay`.
- [x] 2.4 Confirm latest-reading WebSocket updates remain independent from the server-rendered history snapshot and that range navigation uses normal page requests; verify existing and updated WebSocket/detail tests with `go test ./internal/webdisplay`.

## 3. Documentation and Verification

- [x] 3.1 Update the human-facing dashboard documentation with the preset meanings, `1d` default, UTC/calendar semantics, and newest-1,000 behavior; verify the documented URLs and labels match the handler and template tests.
- [x] 3.2 Run `mise run fmt`, `mise run test`, and `mise run vet`, fixing any regressions and verifying the complete supported test suite and static analysis pass.
- [x] 3.3 Run `mise exec -- openspec validate add-history-range-presets --strict`, sync the two delta specifications into their main specifications, and verify the change remains valid and all implementation tasks are checked before archive.
