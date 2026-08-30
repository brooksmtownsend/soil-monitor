## Context

The money-tree firmware already separates continuous wall-power behavior from
battery behavior. Continuous mode waits with `time.sleep()`, while battery mode
calls CircuitPython deep sleep without any wake alarm. CircuitPython restarts
`code.py` from the beginning after a timed deep-sleep alarm, which naturally
re-enters the existing measurement and publication lifecycle.

The existing `PUBLISH_INTERVAL_SECONDS` setting already validates a positive
interval and controls continuous mode. See the delta specification for the
updated observable behavior.

## Goals / Non-Goals

**Goals:**

- Use one interval setting consistently in both power modes.
- Make 3600 seconds the checked-in and fallback interval.
- Schedule the same next wake after successful and failed cycles.
- Preserve bounded network attempts and Wi-Fi shutdown before sleeping.

**Non-Goals:**

- Power-gating the capacitive sensor or measuring battery voltage.
- Aligning publications to wall-clock hour boundaries.
- Preserving Python runtime state across deep sleep.

## Decisions

### Schedule a relative CircuitPython time alarm

Battery mode will create an `alarm.time.TimeAlarm` for
`time.monotonic() + PUBLISH_INTERVAL_SECONDS` immediately before entering deep
sleep and pass it to `alarm.exit_and_deep_sleep_until_alarms()`. Relative time is
appropriate because the requirement is an interval between cycles, not a
wall-clock schedule, and it does not depend on network time synchronization.

The alternative of continuing to use `time.sleep()` was rejected because it
keeps substantially more of the processor active. An external RTC was rejected
because the ESP32-S3 alarm API already supplies the required wake source.

### Reuse the existing interval setting

`PUBLISH_INTERVAL_SECONDS` will control the wait in continuous mode and the
deep-sleep duration in battery mode. Its fallback and checked-in example will
change from 30 to 3600 seconds. Reusing the setting avoids a migration and keeps
short USB tests possible by temporarily selecting a smaller positive value.

A separate battery-only setting was rejected because there is only one active
power mode at a time and both settings express the same measurement cadence.

### Treat every completed attempt identically

Both successful publications and bounded measurement or network failures will
schedule the next timed wake. This preserves recovery from temporary failures
without introducing an awake retry loop.

## Risks / Trade-offs

- **USB-connected deep sleep can be simulated by CircuitPython** → Document that
  a short interval can validate control flow over USB, while true deep sleep
  should be verified using USB power without an active data connection or the
  eventual battery supply.
- **Actual publish cadence exceeds the interval by startup and network time** →
  Define the interval from the point deep sleep begins; wall-clock alignment is
  explicitly outside scope.
- **The sensor remains powered from the 3.3 V rail during sleep** → Keep sensor
  power gating as separately scoped battery-life work and avoid claiming whole
  node current based only on the board's deep-sleep current.

## Migration Plan

Copy the updated firmware and settings example to the board, first using a short
interval such as 60 seconds with battery mode selected. After observing repeated
MQTT publications, set the interval to 3600 seconds and verify true sleep using
power without an active USB data connection. Rollback restores the previous
firmware, whose battery mode sleeps until reset or power cycle.
