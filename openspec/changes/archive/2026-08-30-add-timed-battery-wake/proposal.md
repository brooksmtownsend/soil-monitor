## Why

One-shot battery mode currently enters deep sleep without a wake source, so the
publisher cannot take periodic readings unless it is manually reset or power
cycled. The deployed sensor needs to wake, measure, and publish once per hour
while retaining the short-interval configuration option needed for USB testing.

## What Changes

- Use the configured publication interval as a timed deep-sleep wake interval in
  one-shot battery mode.
- Default and document the firmware interval as one hour (3600 seconds), while
  allowing a shorter value during functional testing.
- Restart the normal measure-and-publish lifecycle after each timed wake,
  including after a bounded measurement or network failure.
- Keep continuous wall-power mode available and keep sensor power gating outside
  this change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `wireless-moisture-publisher`: Replace indefinite one-shot deep sleep with a
  configurable timed wake cycle that defaults to one hour.

## Impact

The change affects the money-tree CircuitPython firmware, its board-local
settings example and operator documentation, and the wireless publisher tests.
It uses CircuitPython's existing `alarm.time.TimeAlarm` API and adds no runtime
dependency.
