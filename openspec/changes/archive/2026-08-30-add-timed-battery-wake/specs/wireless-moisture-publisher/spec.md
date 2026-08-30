## MODIFIED Requirements

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
