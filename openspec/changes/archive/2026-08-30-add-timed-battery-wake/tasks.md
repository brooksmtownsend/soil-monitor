## 1. Timed Wake Firmware

- [x] 1.1 Change the firmware interval fallback to 3600 seconds and schedule a
  `TimeAlarm` before battery-mode deep sleep after both successful and failed
  cycles; verify the CircuitPython files pass host-side syntax compilation.
- [x] 1.2 Preserve continuous-mode waiting with the shared interval and verify
  the existing payload fixture still passes.

## 2. Board Configuration and Guidance

- [x] 2.1 Change the checked-in settings example to periodic battery mode with a
  3600-second interval and verify its comments distinguish short USB testing
  from the deployed hourly cadence.
- [x] 2.2 Update the firmware README with timed wake behavior, USB simulated-sleep
  caveats, and the short-interval-to-hourly verification sequence; verify the
  documented settings match the example file.

## 3. Validation

- [x] 3.1 Run the firmware fixture, `mise run vet`, and strict OpenSpec
  validation for `add-timed-battery-wake`, resolving any failures.
