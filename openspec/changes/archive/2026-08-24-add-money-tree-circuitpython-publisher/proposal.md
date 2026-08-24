# Change: Add the money-tree CircuitPython publisher

## Why

The display appliance can receive MQTT moisture readings, but the repository has
no firmware for the wireless sensor node that will produce them. A USB-powered
test publisher is needed before sensor wiring, calibration, and battery behavior
can be developed safely.

## What Changes

- Add CircuitPython firmware for the Adafruit QT Py ESP32-S3 8 MB/no-PSRAM board.
- Keep Wi-Fi credentials outside source control in `settings.toml`.
- Read the capacitive sensor on `A0`, convert configurable dry/wet endpoints to
  a moisture percentage, and publish it retained to the Pi-local broker.
- Document the CircuitPython libraries and board-copy layout required for the
  sensor-node bench test.

## Impact

- Adds a new `wireless-moisture-publisher` capability.
- Adds files under `firmware/money-tree/`; the existing Go receiver and its
  deployment behavior remain unchanged.
