# Money-tree CircuitPython moisture publisher

This firmware averages the capacitive sensor's analog output, converts it to a
moisture percentage using dry and wet calibration values, and publishes the
percentage to `plants/money-tree/moisture` with MQTT retain enabled. Its operating
mode is selected in `settings.toml`:

- `CONTINUOUS_MODE = true` measures and publishes repeatedly on wall/USB power.
- `CONTINUOUS_MODE = false` runs once, then deep sleeps until reset or power-cycle.

The sensor wiring is:

| Sensor | QT Py ESP32-S3 |
| --- | --- |
| `GND` / black | `GND` |
| `VCC` / red | `3V` |
| `AOUT` / yellow | `A0` |

## CIRCUITPY layout

Copy `code.py` to the root of the `CIRCUITPY` drive. Copy
`settings.toml.example` there as `settings.toml`, then replace the two Wi-Fi
placeholders. The finished board should contain:

```text
CIRCUITPY/
  code.py
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
Published retained moisture: 50 %
Next measurement in 30 seconds.
```

`DRY_VALUE` maps to 0%, `WET_VALUE` maps to 100%, and readings outside that
range are clamped. The checked-in values are preliminary breadboard measurements;
repeat calibration after the headers and final sensor wiring are secure.

MiniMQTT makes at most three connection attempts per measurement cycle. In
continuous mode, a failure turns off Wi-Fi and waits for
`PUBLISH_INTERVAL_SECONDS` before starting a fresh cycle. In one-shot mode, a
failure enters deep sleep. Set `CONTINUOUS_MODE = false` before moving the node
to battery power; a timed battery wake schedule remains deferred.
