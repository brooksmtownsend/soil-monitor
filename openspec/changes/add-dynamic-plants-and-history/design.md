## Context

The process currently builds an in-memory `core.State` from exactly three configured `id:name` pairs. MQTT messages carry only a percentage, `app.Runner` serializes MQTT and button events, the OLED assumes at least one plant, and the embedded HTTP server renders a read-only snapshot. Restarting loses readings, and the CircuitPython publisher's retained integer is the only recovery mechanism.

This change crosses configuration, core state, application coordination, MQTT payload handling, HTTP behavior, deployment, and publisher firmware. The resulting Linux ARM64 binary must remain static and self-contained, the browser must not depend on external assets, state mutation must remain serialized through the application layer, and the trusted-LAN/no-TLS deployment boundary remains unchanged.

## Goals / Non-Goals

**Goals:**

- Discover valid plant IDs from `plants/+/moisture` without a predefined count.
- accept and validate `{ "raw": <integer>, "percentage": <integer> }` payloads;
- durably store plant metadata and every accepted reading before publishing it to in-memory views;
- restore the discovered catalog and latest values at startup;
- let users assign durable names and inspect per-plant history in the embedded browser;
- preserve deterministic OLED navigation when the catalog is empty or changes; and
- keep release artifacts static for Linux ARM64 and keep all browser assets embedded.

**Non-Goals:**

- Authentication, TLS, remote database hosting, cloud synchronization, alerts, watering automation, or multi-user authorization.
- Editing plant IDs, deleting plants/readings, data export, configurable retention, or historical aggregation/downsampling in this change.
- JavaScript charting libraries or a new frontend build/runtime.
- Backward compatibility with the legacy integer MQTT payload.

## Decisions

### Use SQLite behind a narrow history repository

Add an `internal/history` package with a repository interface consumed by the application layer and a SQLite implementation. Use a pure-Go SQLite driver so `CGO_ENABLED=0` and the existing Linux ARM64 static build remain supported. The database opens during composition, enables foreign keys and a bounded busy timeout, applies idempotent schema migrations in a transaction, and closes during shutdown.

The initial schema has a `plants` table keyed by plant ID with nullable assigned name and first/last-seen UTC timestamps, plus a `readings` table containing an auto-increment key, plant ID, application receipt timestamp, raw integer, and percentage integer. An index on `(plant_id, observed_at, id)` supports ordered chart queries. SQLite constraints mirror application validation for percentage and non-negative raw values.

Alternatives considered were a JSON/CSV append log, which makes atomic name updates and indexed queries awkward, and a CGO-backed SQLite driver, which conflicts with the static cross-build contract.

### Treat application receipt time as the observation timestamp

The requested payload contains raw and percentage values but no trustworthy clock. The monitor therefore timestamps an accepted message in UTC when processing it. Retained deliveries after a restart are recorded as new observations because the protocol provides no stable reading identifier with which to deduplicate them.

Adding a publisher timestamp or UUID was considered, but battery publishers may not have a reliable real-time clock and the requested wire contract does not include either field.

### Validate, commit, then update the in-memory projection

Parsing produces a typed reading only when the topic is exactly `plants/<kebab-case-id>/moisture`, the JSON contains integer `raw` and `percentage` fields, raw is non-negative, percentage is in `0..100`, and no trailing JSON value exists. Unknown JSON fields are tolerated for forward compatibility. The runner asks the repository to transactionally upsert the plant and append its reading, then updates `core.State`. If persistence fails, the latest in-memory value is not advanced and the error is logged, keeping the displayed value consistent with durable history.

`core.State` becomes a dynamic, thread-safe projection loaded from the repository at startup. Newly observed plants are appended in first-seen order, with plant ID as a stable tie-breaker when restoring. A plant without an assigned name displays its ID. State snapshots include raw value, percentage, and observation time. The currently selected ID is preserved when other plants arrive; the first discovered plant becomes selected; buttons are no-ops and the OLED renders an explicit no-plants view while the catalog is empty.

Keeping SQLite calls inside `core.State` was rejected because it would mix domain projection and persistence concerns and make state tests database-dependent.

### Serialize browser name changes through the application runner

The HTTP server receives a narrow command interface implemented by the application layer. `POST /plants/{escaped-id}/name` validates a form-encoded, trimmed Unicode display name of 1 through 80 characters, sends a rename command to the runner, and waits for its result. The runner persists the name first and then updates `core.State`, preserving its role as the mutation coordinator. Unknown plants return 404, invalid names return 400, persistence failures return 500, and success redirects with 303 to the plant page.

The interface remains unauthenticated because the deployment contract explicitly treats port 8080 as a trusted-LAN service. The documentation will call out that name changes are now possible on that listener. Direct web-handler mutation of state was rejected because it bypasses serialized coordination.

### Render history as server-side inline SVG

`GET /` remains the status overview and links each discovered plant to `GET /plants/{escaped-id}`. The detail page includes current raw/percentage values, the rename form, and an accessible inline SVG percentage chart generated from at most the newest 1,000 stored readings and then ordered chronologically. Axes cover 0 through 100 percent; a tabular fallback lists timestamps, raw values, and percentages. HTML templates, CSS, and SVG remain embedded in the Go binary with no external assets or JavaScript.

The repository retains all readings; only rendering is bounded. A fixed query limit prevents a large database from causing unbounded response memory or SVG size. Client-side chart libraries were rejected because they add an asset/runtime dependency and weaken the existing server-rendered appliance model.

### Put the database in the service's persistent state directory

Add layered `SOIL_DATABASE_PATH` / `--database-path` configuration with default `/var/lib/soil-monitor/soil-monitor.db`. The systemd unit uses `StateDirectory=soil-monitor` so the unprivileged `soilmonitor` process receives a writable, durable directory. Bootstrap creates or normalizes the directory without overwriting an existing database, and routine release never transfers, replaces, or removes database files.

An in-memory default was rejected because it defeats restart persistence. Locating the database under `/etc` was rejected because configuration should remain host-managed and mostly read-only.

### Update publishers and test utilities atomically with the consumer contract

The CircuitPython publisher serializes its averaged ADC reading and calculated percentage as compact JSON and retains that payload on the existing topic. Local publishing examples and blast utilities emit the same object shape. The monitor does not accept the legacy bare integer, making malformed or stale senders visible instead of fabricating a raw value.

## Risks / Trade-offs

- [Unbounded history can grow until storage is exhausted] → Use compact rows and indexed bounded reads, document database monitoring/backup, and leave a configurable retention policy to a later change.
- [SQLite writes add latency to the serialized event loop] → Keep transactions small, use WAL mode and a busy timeout, and perform only one upsert/insert transaction per reading; measure before introducing batching.
- [A retained MQTT message creates another row after restart] → Document receipt-time semantics; a future wire version can add a reading ID if exact deduplication becomes necessary.
- [Unauthenticated name changes expand the trusted-LAN attack surface] → Limit mutation to a small validated route, use escaped identifiers and bounded names, and make the deployment documentation explicit.
- [Pure-Go SQLite increases binary size and dependency surface] → Pin the module, verify licenses and ARM64 builds, and keep database access behind a replaceable repository interface.
- [No plants at startup changes OLED assumptions] → Add explicit empty-state rendering and no-op navigation tests before enabling database-backed startup.

## Migration Plan

1. Add the repository, schema migrations, dynamic projection, and tests without running a live release.
2. Update MQTT parsing, the publisher firmware, examples, and fixtures to the JSON payload as one coordinated protocol change.
3. Update browser routes/templates, runtime configuration, systemd state-directory behavior, and deployment fixtures.
4. Run unit, race, shell, static-analysis, local build, and Linux ARM64 build verification, then strictly validate OpenSpec.
5. Before deployment, back up the environment file and any existing application data, update all publishers, and release the monitor binary. The first start creates the database; plants appear as retained JSON readings arrive and can then be named in the browser.

Rollback requires stopping the service and restoring the prior binary and deployment files. The prior binary cannot consume JSON payloads, so publishers must also be restored to integer payloads. The SQLite database can remain in place for a later upgrade because the old binary does not access it. Existing `SOIL_PLANTS` values may remain in the host environment during rollback but are ignored by the new binary.

## Open Questions

- A configurable retention/downsampling policy should be designed after real reading frequency and storage growth are observed.
- A future versioned payload may add publisher timestamps and stable reading IDs if deduplication or offline sample buffering becomes necessary.
