# wireless-moisture-publisher Specification

## Purpose

Define the USB-powered wireless publisher that reads the money-tree moisture
sensor and reports a calibrated percentage over MQTT.

## ADDED Requirements

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

The publisher SHALL publish the calculated base-10 integer from 0 through 100 to
`plants/money-tree/moisture` using MQTT retain after connecting to the configured
trusted-LAN broker.

#### Scenario: USB-powered board boots with valid configuration

- **WHEN** the board joins Wi-Fi and connects to the broker
- **THEN** it publishes the calculated moisture percentage once with retain enabled
  and reports success through the serial console

### Requirement: Failed setup remains diagnosable

The publisher SHALL report Wi-Fi or MQTT connection failures through the serial
console, disable Wi-Fi, and enter deep sleep without publishing an invalid
payload. Network connection attempts SHALL be bounded.

#### Scenario: Broker is temporarily unavailable

- **WHEN** the board cannot connect to the MQTT broker
- **THEN** it reports the failure and enters deep sleep until reset or power-cycle

### Requirement: Configurable power mode

The publisher SHALL support a board-local setting that selects continuous
wall-power operation or one-shot battery operation.

#### Scenario: Continuous wall-power mode is selected

- **WHEN** a measurement cycle succeeds or its bounded network attempt fails
- **THEN** the publisher disables Wi-Fi, waits the configured interval, and
  begins another measurement cycle

#### Scenario: One-shot battery mode is selected

- **WHEN** a measurement cycle succeeds or its bounded network attempt fails
- **THEN** the publisher disables Wi-Fi and enters deep sleep until reset or
  power-cycle
