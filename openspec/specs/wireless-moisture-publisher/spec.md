# wireless-moisture-publisher Specification

## Purpose

Define the USB-powered wireless publisher that reads the money-tree moisture
sensor and reports a calibrated percentage over MQTT.

## Requirements

### Requirement: Board-local network configuration

The publisher SHALL read the Wi-Fi SSID and password from a board-local
`settings.toml` file and SHALL NOT require credentials to be committed with the
firmware source.

#### Scenario: Credentials are absent

- **WHEN** required Wi-Fi settings are missing
- **THEN** the publisher reports the missing setting through the serial console
  without attempting an MQTT publication

### Requirement: Calibrated moisture measurement

The publisher SHALL average the configured number of `A0` readings while Wi-Fi
is disabled, map the configured dry and wet values to 0 and 100 percent, and
clamp results outside that range.

#### Scenario: Sensor reads halfway between calibration endpoints

- **WHEN** the averaged sensor reading is halfway between the configured dry and
  wet values
- **THEN** the publisher calculates a moisture value of 50 percent

### Requirement: Retained money-tree publication

The publisher SHALL publish a compact JSON object containing the non-negative averaged raw sensor reading as integer field `raw` and the calculated integer from 0 through 100 as field `percentage` to `plants/money-tree/moisture` using MQTT retain after connecting to the configured trusted-LAN broker.

#### Scenario: USB-powered board boots with valid configuration

- **WHEN** the board joins Wi-Fi, reads averaged raw value 32768 and calculated percentage 50, and connects to the broker
- **THEN** it publishes `{"raw":32768,"percentage":50}` once with retain enabled and reports success through the serial console

#### Scenario: Continuous mode completes another measurement

- **WHEN** a later continuous-mode cycle obtains a new raw reading and percentage
- **THEN** the publisher replaces the retained topic value with a JSON object containing both new integers

### Requirement: Failed setup remains diagnosable

The publisher SHALL report Wi-Fi or MQTT connection failures through the serial
console, disable Wi-Fi, and enter the configured power-mode wait without
publishing an invalid payload. Network connection attempts SHALL be bounded.

#### Scenario: Broker is temporarily unavailable

- **WHEN** the board cannot connect to the MQTT broker in battery mode
- **THEN** it reports the failure, disables Wi-Fi, and enters deep sleep until
  the configured interval elapses

### Requirement: Configurable power mode

The publisher SHALL support a board-local setting that selects continuous
wall-power operation or periodic battery operation. Both modes SHALL use a
positive configured measurement interval that defaults to 3600 seconds.

#### Scenario: Continuous wall-power mode is selected

- **WHEN** a measurement cycle succeeds or its bounded network attempt fails
- **THEN** the publisher disables Wi-Fi, waits the configured interval without
  entering deep sleep, and begins another measurement cycle

#### Scenario: One-shot battery mode is selected

- **WHEN** a measurement cycle succeeds or its bounded network attempt fails
- **THEN** the publisher disables Wi-Fi and enters deep sleep with a timed wake
  scheduled for the configured interval

#### Scenario: Timed battery wake occurs

- **WHEN** the configured deep-sleep interval elapses
- **THEN** the board restarts the firmware lifecycle and attempts a new
  measurement and publication
