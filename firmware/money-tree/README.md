# Money-tree CircuitPython moisture publisher

This firmware averages the capacitive sensor's analog output, converts it to a
moisture percentage using dry and wet calibration values, and publishes the
raw average and percentage as JSON to `plants/money-tree/moisture` with MQTT
retain enabled. Its operating mode is selected in `settings.toml`:

- `CONTINUOUS_MODE = true` stays awake and repeats after
  `PUBLISH_INTERVAL_SECONDS` on wall/USB power.
- `CONTINUOUS_MODE = false` measures and publishes once, enters deep sleep for
  `PUBLISH_INTERVAL_SECONDS`, then wakes by restarting the firmware lifecycle.

The checked-in configuration selects periodic battery mode with a one-hour
interval (`PUBLISH_INTERVAL_SECONDS = 3600`). The interval begins when deep
sleep starts, so startup and network connection time are additional to the
configured sleep duration.

The sensor wiring is:

| Sensor | QT Py ESP32-S3 |
| --- | --- |
| `GND` / black | `GND` |
| `VCC` / red | `3V` |
| `AOUT` / yellow | `A0` |

## CIRCUITPY layout

Copy `code.py` and `payload.py` to the root of the `CIRCUITPY` drive. Copy
`settings.toml.example` there as `settings.toml`, then replace the two Wi-Fi
placeholders. The finished board should contain:

```text
CIRCUITPY/
  code.py
  payload.py
  settings.toml
  lib/
    adafruit_connection_manager.mpy
    adafruit_ticks.mpy
    adafruit_minimqtt/
```

Download the Adafruit library bundle matching the board's CircuitPython major
version from <https://circuitpython.org/libraries>. Copy the three library items
shown above from the bundle's `lib/` directory into `CIRCUITPY/lib/`.

Saving `code.py` starts it automatically. Open the CircuitPython serial console
to see Wi-Fi and MQTT diagnostics. A successful run ends with:

```text
Raw average: 44536
Moisture: 50 %
Published retained moisture: {"raw":44536,"percentage":50}
Measurement complete; entering timed deep sleep.
Next measurement in 3600 seconds.
```

`DRY_VALUE` maps to 0%, `WET_VALUE` maps to 100%, and readings outside that
range are clamped. The checked-in values are preliminary breadboard measurements;
repeat calibration after the headers and final sensor wiring are secure.

MiniMQTT makes at most three connection attempts per measurement cycle. In
continuous mode, a failure turns off Wi-Fi and waits for
`PUBLISH_INTERVAL_SECONDS` before starting a fresh cycle. In periodic battery
mode, either success or failure turns off Wi-Fi and schedules the next timed
deep-sleep wake.

## Test the timed wake

Start with these values in the board's `settings.toml`:

```toml
CONTINUOUS_MODE = false
PUBLISH_INTERVAL_SECONDS = 60
```

Confirm that multiple readings arrive about one minute apart. When the board is
connected to a computer over USB, CircuitPython may simulate deep sleep to keep
the serial console and `CIRCUITPY` drive available. This is useful for checking
the repeated control flow, but it does not demonstrate low-power operation.

For a true deep-sleep test, power the USB-C port from a wall adapter or through a
USB data blocker and observe the MQTT reading history. After that succeeds,
change `PUBLISH_INTERVAL_SECONDS` to `3600` for the deployed hourly cadence.
CircuitPython restarts `code.py` from the beginning after every timed wake.

The capacitive sensor remains connected to the always-powered `3V` rail during
deep sleep. Sensor power gating and whole-node current optimization remain
separate battery-life work.
