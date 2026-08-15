## Why

The project needs a small, testable foundation that proves a Raspberry Pi 3A+ can receive plant moisture values over MQTT and present them on the Waveshare 1.3-inch OLED HAT. Defining this MVP first keeps the initial build focused while leaving persistence, history, and direct sensor integration for later changes.

## What Changes

- Add a single Go application that subscribes to MQTT moisture topics for three configured plants.
- Accept a numeric moisture value on a per-plant topic and retain the latest value in memory.
- Show the selected plant and its latest moisture value on the SH1106 OLED.
- Use the HAT's three buttons to move between plants and reserve one button for a future action.
- Show useful startup, waiting, and disconnected states without requiring stored history.
- Document a minimal runtime configuration and a laptop-to-Pi publishing workflow for bench testing.
- Exclude historical storage, charts, alerts, authentication, remote configuration, and direct sensor reads from this MVP.

## Capabilities

### New Capabilities

- `plant-moisture-display`: Receive current moisture values for configured plants over MQTT and browse those values on the attached OLED using physical buttons.

### Modified Capabilities

None.

## Impact

- Introduces a Go module and one deployable application for the Raspberry Pi 3A+.
- Integrates with an MQTT broker through a Go MQTT client.
- Integrates with the Waveshare 1.3-inch SH1106 OLED and BCM GPIO buttons 21, 20, and 16.
- Requires SPI and GPIO access on 64-bit Debian Lite and configuration for broker connection and three plant identifiers.
- Does not change any existing API, application, or specification.
