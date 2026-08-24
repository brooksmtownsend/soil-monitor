# Design: Money-tree CircuitPython test publisher

## Context

The first sensor node is an Adafruit QT Py ESP32-S3 with 8 MB flash and no PSRAM.
It is currently powered over USB-C with a capacitive moisture probe on `A0`.
The Raspberry Pi broker is available on the trusted LAN at `192.168.1.95:1883`.

## Decisions

CircuitPython will be used for the first node so configuration and firmware can
be copied directly to the `CIRCUITPY` volume. Wi-Fi credentials will live only in
the board-local `settings.toml`; the repository contains a placeholder example.

The firmware will disable Wi-Fi while taking 20 analog samples because `A0` uses
ADC2, average the samples, and linearly map configurable dry and wet endpoints to
0% and 100%. It will clamp values outside that calibrated range. It will then
enable Wi-Fi and publish the computed percentage once per boot with QoS 1 and
MQTT retain enabled before becoming idle. This resembles the eventual battery
lifecycle, where a node wakes, measures, publishes, and sleeps.

Network attempts are bounded. MiniMQTT may attempt its connection three times,
but the application will not spin immediately on failure. A board-local setting
selects continuous wall-power operation or one-shot battery operation. Continuous
mode disables Wi-Fi and waits for its configured interval before starting a fresh
measurement cycle. One-shot mode enters deep sleep without a wake alarm after
success or failure; reset or power-cycle begins the next measurement. A timed
battery wake alarm is deferred to the battery measurement-interval work.

MiniMQTT will be installed from the Adafruit CircuitPython library bundle rather
than vendored into this repository. Its ConnectionManager and ticks dependencies
must accompany it on the board.

## Deferred work

- Deep sleep and sensor power gating.
- Battery-voltage observation and low-battery behavior.
- Firmware for the other two configured plants.
