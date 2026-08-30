## Why

Plant detail pages always show the newest stored readings without letting a user focus on a meaningful period. Preset rolling ranges make recent moisture trends easier to interpret while keeping the interface small and predictable.

## What Changes

- Add `1d`, `1w`, `1mo`, `3mo`, `6mo`, and `1y` rolling history ranges ending at the current request time.
- Add range controls to each plant detail page and preserve the selected range in the page URL.
- Filter stored readings by the selected range before applying the existing newest-1,000-reading browser limit.
- Define deterministic default and invalid-range behavior for direct HTTP requests.
- Keep plant metadata, stored readings, MQTT ingestion, and retention behavior unchanged; history deletion is outside this change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `plant-reading-history`: Historical queries gain a UTC time interval in addition to their existing plant and result-limit constraints.
- `local-web-display`: Plant detail pages gain preset range selection and render only readings in the selected rolling interval.

## Impact

- `internal/history/`: extend the repository query contract and SQLite query to filter by observation time.
- `internal/webdisplay/`: parse and validate the range query, calculate rolling boundaries, render range controls, and communicate the active/empty range.
- Tests for storage boundary behavior, HTTP defaults and validation, range links, and filtered charts/tables.
- No database migration, new runtime dependency, external asset, or deployment configuration change is expected.
