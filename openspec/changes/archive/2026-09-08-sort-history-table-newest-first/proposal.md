## Why

The history table currently places the oldest observation at the top, forcing users to scan or scroll to find the latest reading, while dashboard timestamps are displayed in UTC instead of the operator's Eastern time. Showing the newest observations first and presenting human-readable times in DST-aware Eastern Time makes recent readings easier to interpret without changing which readings are selected or how they are stored.

## What Changes

- Render plant history table rows in descending observation order so the newest reading appears first.
- Use deterministic reverse receipt order when multiple readings have the same observation timestamp.
- Keep the inline SVG chart in chronological order from oldest to newest so time continues to progress left to right.
- Display human-readable observation and range-boundary timestamps in `America/New_York`, using `EST` or `EDT` as appropriate for the instant.
- Apply the same Eastern display conversion to initial server-rendered pages and subsequent WebSocket updates.
- Keep machine-readable timestamp values, SQLite storage, and range calculations as absolute UTC instants.
- Keep range filtering, the newest-1,000-reading limit, stored query order, and persistence behavior unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `local-web-display`: The plant history table changes from oldest-first to newest-first while its chart remains chronological, and all human-readable dashboard timestamps use DST-aware Eastern Time.

## Impact

- `internal/webdisplay/`: provide a reverse-ordered table view without mutating the chronological reading slice, and share one Eastern display conversion across HTML and WebSocket rendering.
- `internal/webdisplay` tests: assert newest-first table rows, unchanged chronological chart points, and correct `EST`/`EDT` conversion.
- `README.md`: document Eastern display behavior while retaining UTC storage and query semantics.
- The standard-library time-zone database will be embedded so static deployments do not depend on host zoneinfo files; no external dependency, database schema, configuration, deployment, or API change is expected.
