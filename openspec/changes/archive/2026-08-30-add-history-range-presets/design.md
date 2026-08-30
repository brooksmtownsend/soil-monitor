## Context

See `proposal.md` for motivation. History is stored in SQLite as UTC RFC 3339 timestamps, with an existing `(plant_id, observed_at, id)` index. The web detail handler currently requests the newest 1,000 readings through a repository interface, then renders one inline SVG and table. Range selection crosses the web and storage modules but does not require a schema or deployment change.

## Goals / Non-Goals

**Goals:**

- Keep preset parsing and rolling-boundary calculation deterministic and directly testable.
- Apply time filtering in SQLite so older rows are not loaded and discarded by the HTTP layer.
- Preserve deterministic ordering and the existing 1,000-row response bound.
- Preserve the server-rendered, self-contained dashboard architecture.

**Non-Goals:**

- Arbitrary start/end date inputs, user time-zone selection, aggregation, sampling, or data deletion.
- Changing stored timestamp representation, retention policy, or database schema.
- Dynamically replacing chart history through the live-update WebSocket; a range change remains a normal page navigation.

## Decisions

### Represent a selection as a closed UTC interval

The detail handler will capture its clock once per request as `end`, convert it to UTC, and derive `start` from the selected preset. `1d` and `1w` subtract 24 hours and 7 days. Month and year presets use a small calendar helper for 1, 3, 6, or 12 months that preserves the UTC time of day and clamps the original day to the target month's last valid day. The same captured `end` is passed to storage so calculation, query, and page labels cannot drift across separate clock reads.

Alternative: treat every preset as a fixed number of days. This was rejected because calendar-month labels would then cover surprising periods and diverge around month lengths and leap years.

### Validate a single `range` query value in the web layer

The server will map the six public tokens to labels and boundary functions. A missing query value selects `1d`; an explicitly empty or unknown value returns HTTP 400 before the history repository is called. Links use canonical URLs such as `/plants/money-tree?range=3mo`, and template data marks exactly one option as current.

Alternative: silently fall back to `1d` for invalid values. This was rejected because it hides malformed bookmarks and makes the displayed result differ from the requested URL.

### Extend the repository query with start and end instants

The history repository contract will accept a plant identifier, inclusive `start` and `end` values, and a positive limit. Storage will reject zero or reversed intervals, canonicalize bounds to UTC, filter `observed_at` between both bounds, select the newest qualifying rows up to the limit, and restore chronological order in the outer query. The existing composite index aligns with plant, time, and deterministic ID ordering, so no migration is planned.

Alternative: fetch the newest 1,000 readings first and filter in Go. This was rejected because it can produce an empty or incomplete long-range result even when qualifying older readings exist. Fetching all history and filtering in Go was rejected because it removes the response bound from the storage workload.

### Inject the web clock at the server boundary

The server will hold a clock function initialized to `time.Now`. Package tests can replace it with a fixed instant to verify each preset and exact inclusivity without changing the public constructor or relying on wall-clock timing.

Alternative: call `time.Now` directly in the handler. This was rejected because calendar boundaries and near-boundary tests would be flaky.

### Keep the chart and table structurally unchanged

Range controls and an active-range/empty-range description will be added around the existing chart and table. The same filtered reading slice continues to drive both representations, ensuring their contents agree. The existing live-update mechanism may update latest-reading fields, but history remains the snapshot selected when the detail page was requested.

Alternative: push complete filtered histories through WebSockets. This was rejected because it broadens the protocol and client state substantially without being necessary for range navigation.

## Risks / Trade-offs

- [A one-year interval may contain far more than 1,000 readings] → Keep the existing newest-1,000 cap visible in the table caption and range description.
- [Calendar subtraction near month-end can produce a normalized date users do not expect] → Clamp to the target month's final valid day and cover month-end and leap-year cases with fixed-clock tests.
- [The detail chart does not automatically append new history points while open] → Keep current snapshot semantics; selecting or reloading a range refreshes its history, while latest-reading live updates remain independent.
- [Invalid interval parameters could lead to misleading storage results] → Validate non-zero bounds, `start <= end`, and a positive limit at the repository boundary.

## Migration Plan

No database migration is required. Deploy the updated binary normally; existing readings and URLs remain valid, with range-less detail URLs now selecting `1d`. Rollback consists of restoring the prior binary, which will ignore range query values and continue reading the unchanged database.
