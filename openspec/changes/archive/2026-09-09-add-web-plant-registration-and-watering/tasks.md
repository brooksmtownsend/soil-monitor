## 1. Domain Model and Validation

- [x] 1.1 Add watering cadence, local due-date, photo metadata, and care fields to history and core plant models; verify model and copy behavior with focused `internal/core` tests.
- [x] 1.2 Implement deterministic display-name-to-ID normalization, including Latin diacritic removal and conflict-ready validation, and verify mixed case, punctuation, Unicode, empty results, and length limits with table-driven tests.
- [x] 1.3 Implement weekly, biweekly, monthly-clamped, as-needed, and unconfigured next-date calculation in `America/New_York`; verify end-of-month, leap-year, overdue, and daylight-saving boundary cases with unit tests.
- [x] 1.4 Implement bounded JPEG, PNG, and HEIC signature detection and canonical media types; verify valid signatures, misleading filenames/types, truncated content, unsupported brands, empty input, and the 20 MiB boundary with unit tests.

## 2. Durable Care Storage

- [x] 2.1 Add idempotent `plant_care` and `waterings` companion-table creation without changing schema version 1; verify fresh, existing version-1, repeated-open, and newer-version rejection migration tests.
- [x] 2.2 Extend plant restoration queries with cadence, next date, and `HasPhoto` without selecting blob bytes; verify legacy plants restore as unscheduled and registered plants restore with care metadata and latest readings.
- [x] 2.3 Add transactional repository operations for registration, schedule changes, watering confirmation, photo replacement, and photo retrieval, with domain errors for conflicts and missing plants; verify success, rollback-on-failure, unchanged prior photos, and watering-event persistence in store tests.
- [x] 2.4 Verify the additive migration remains readable by a version-1-compatible query path and that existing plants, names, and reading history are unchanged after migration.

## 3. Serialized Application Coordination

- [x] 3.1 Extend core state to add committed registered plants and apply committed care/photo updates while preserving first-seen ordering and selection; verify first registration selection, later append behavior, no-reading placeholders, and MQTT updates of registered plants.
- [x] 3.2 Replace the rename-only command boundary with typed registration, rename, schedule, watering, and photo commands that validate inputs and respect request cancellation; verify commander unit tests for validation, results, and unavailable/canceled coordination.
- [x] 3.3 Handle all web mutations in the runner by persisting before state updates, then issuing appropriate OLED redraws and web notifications; verify integration tests cover success, persistence failure, identifier conflicts, concurrent MQTT updates, and preservation of care metadata.

## 4. Web Routes and Upload Handling

- [x] 4.1 Add bounded multipart parsing with whole-request limits, a 20 MiB photo-field limit, temporary-file cleanup, and a single concurrent upload slot; verify accepted boundary uploads and early rejection/cleanup for oversized or malformed requests.
- [x] 4.2 Add `GET /plants/new` and multipart `POST /plants` with the specified 303, 400, 409, 413, 415, and 500 outcomes; verify registration with and without photos, normalized redirects, conflicts with registered and MQTT-discovered IDs, and atomic failure behavior in handler tests.
- [x] 4.3 Add schedule and watering POST routes and connect them to serialized commands; verify scheduled, as-needed, unconfigured, overdue, unknown-plant, invalid-cadence, and persistence-failure handler cases.
- [x] 4.4 Add photo GET/HEAD and replacement POST behavior with stored content type, content length, no-store caching, and nosniff protection; verify missing photos, all supported types, no HEAD body, replacement preservation, method rejection, and status mappings.

## 5. Web Presentation

- [x] 5.1 Update the overview template with a registration action, registered no-reading plants, and optional identifying photos; verify rendered HTML for empty, photographed, and sensorless catalogs while preserving MQTT status and live-reading hooks.
- [x] 5.2 Update the plant detail template to present the display name, photo, cadence, next watering date or explicit as-needed/unconfigured state, confirm-watering action, schedule/photo forms, latest reading, and existing moisture history; verify server-rendered output across all care states and history ranges.
- [x] 5.3 Extend lightweight WebSocket snapshots with care and photo-presence fields needed by open pages, never image bytes; verify snapshot serialization and race-safe concurrent notification tests.
- [x] 5.4 Add responsive photo and form styling without external assets or libraries; verify template parsing and mobile-width layout through existing render tests or captured HTML assertions.

## 6. Documentation and Verification

- [x] 6.1 Update operator documentation for web registration, derived immutable IDs and conflicts, schedule calculations, watering confirmation, 20 MiB JPEG/PNG/HEIC uploads, HEIC browser limitations, trusted-LAN mutations, backups, and the register-before-publish MQTT workflow; verify every new route and behavior has a documented exercise.
- [x] 6.2 Run the focused package tests throughout implementation, then run `mise run fmt`, `mise run test`, and `mise run vet`; verify all commands complete successfully.
- [x] 6.3 Run `go test -race ./...` because command and snapshot concurrency changed, and verify it reports no races on the local platform.
- [x] 6.4 Run `mise run build-pi` and strict OpenSpec validation for `add-web-plant-registration-and-watering`; verify the static Linux ARM64 binary builds and every change artifact and delta spec passes validation.
