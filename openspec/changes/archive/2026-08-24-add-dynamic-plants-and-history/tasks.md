## 1. Persistence Foundation

- [x] 1.1 Add and pin a pure-Go SQLite dependency, then verify its license and `CGO_ENABLED=0` Linux ARM64 compatibility.
- [x] 1.2 Define the history repository models and interfaces for catalog restoration, atomic reading append, name update, and bounded chronological queries.
- [x] 1.3 Implement SQLite open options, transactional idempotent schema migrations, constraints, indexes, busy timeout, WAL mode, and clean shutdown.
- [x] 1.4 Implement and test atomic plant upsert plus reading insertion, catalog/latest restoration order, name updates, unknown-plant handling, and newest-1,000 history queries.
- [x] 1.5 Add failure-path tests proving migration errors and failed commits do not expose uncommitted catalog, name, or latest-reading state.

## 2. Dynamic Domain State and MQTT Readings

- [x] 2.1 Replace fixed plant definitions with a dynamic `core.State` projection that supports an empty catalog, deterministic restored order, identifier label fallback, and stable selection as plants arrive.
- [x] 2.2 Extend plant views and snapshots with raw value, percentage, and UTC receipt time, and test defensive copying and concurrent access.
- [x] 2.3 Generalize previous/next navigation and OLED view generation for zero, one, and arbitrary plant counts, including wraparound and name truncation tests.
- [x] 2.4 Implement strict topic and JSON reading parsing for required integer `raw` and `percentage` fields, allowed unknown fields, rejected trailing data, kebab-case IDs, and range validation.
- [x] 2.5 Update the application runner to timestamp, persist, and then project valid MQTT readings; log and preserve prior state on parse or storage failure; and redraw when selection-visible state changes.
- [x] 2.6 Add runner and race-focused tests for first-plant discovery, non-selected updates, retained restart restoration, invalid messages, persistence failures, and concurrent MQTT/button/snapshot activity.

## 3. Serialized Plant Naming

- [x] 3.1 Add the application command channel/interface for display-name changes with 1-to-80 trimmed Unicode character validation and result propagation.
- [x] 3.2 Persist rename commands before updating `core.State`, and test successful replacement, unknown plants, invalid names, database failures, and concurrent rename/MQTT ordering.
- [x] 3.3 Wire the rename command interface through process composition without allowing the HTTP adapter to mutate state or storage directly.

## 4. Historical Web Interface

- [x] 4.1 Refactor the embedded HTTP router and overview template for empty catalogs, identifier fallback names, latest raw/percentage/time fields, and escaped plant-detail links.
- [x] 4.2 Implement `GET`/`HEAD /plants/{id}` with bounded repository history lookup, latest status, unknown-plant handling, and method restrictions.
- [x] 4.3 Build an accessible server-rendered inline SVG chart with a fixed 0-to-100 percentage scale and a timestamped raw/percentage table for up to 1,000 chronological readings.
- [x] 4.4 Implement form-encoded `POST /plants/{id}/name` with validation, serialized command dispatch, 303 redirect, and specified 400/404/500 responses.
- [x] 4.5 Add handler and concurrency tests covering overview refresh, empty state, chart bounds/order, HTML/path escaping, name changes, unsupported routes/methods, storage errors, and race-free snapshots.

## 5. Configuration and Process Lifecycle

- [x] 5.1 Remove fixed `SOIL_PLANTS` parsing and validation from configuration, examples, composition, and tests while retaining validation for dynamically discovered kebab-case IDs.
- [x] 5.2 Add layered `SOIL_DATABASE_PATH` / `--database-path` configuration with the `/var/lib/soil-monitor/soil-monitor.db` default, validation, help text, and precedence tests.
- [x] 5.3 Open, migrate, restore, inject, and close the history repository in the composition root with startup diagnostics and cleanup on every partial-initialization failure.
- [x] 5.4 Update application lifecycle tests for empty startup, restored startup, database initialization failure before adapters, and graceful repository shutdown.

## 6. Publisher and Operator Utilities

- [x] 6.1 Update the money-tree CircuitPython publisher to retain compact JSON containing averaged raw and calculated percentage integers, with firmware tests or host-side fixtures for exact payload shape.
- [x] 6.2 Update `scripts/blast.sh`, MQTT examples, and test fixtures to publish valid JSON readings and remove fixed-plant prerequisites.
- [x] 6.3 Update bench-test and runtime documentation for dynamic discovery, name assignment, charts, receipt-time/retained-message semantics, the breaking payload migration, and database backup/size monitoring.

## 7. Deployment Persistence

- [x] 7.1 Add `StateDirectory=soil-monitor` and the database setting to systemd and environment assets while preserving the unprivileged service boundary.
- [x] 7.2 Update bootstrap logic and fixtures to create or normalize the persistent state directory without overwriting an existing database.
- [x] 7.3 Update release logic and fixtures to prove routine releases do not transfer, replace, change metadata on, or remove the SQLite database.
- [x] 7.4 Document coordinated upgrade and rollback steps for monitor and publishers, including preservation of the new database and temporary legacy `SOIL_PLANTS` values.

## 8. Verification and Specification Completion

- [x] 8.1 Run formatting and directly affected package and firmware/script fixture tests, fixing all failures.
- [x] 8.2 Run `mise run test`, `mise run vet`, `mise run lint-shell`, and `go test -race ./...` on a supported local platform.
- [x] 8.3 Run local and static Linux ARM64 builds and verify the resulting appliance binary has no runtime CGO or external frontend asset dependency.
- [x] 8.4 Run strict OpenSpec validation, sync the five delta specifications into the main specs without dropping unrelated requirements, and verify all implementation tasks are checked before archive.
