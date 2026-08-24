## MODIFIED Requirements

### Requirement: Retained money-tree publication
The publisher SHALL publish a compact JSON object containing the non-negative averaged raw sensor reading as integer field `raw` and the calculated integer from 0 through 100 as field `percentage` to `plants/money-tree/moisture` using MQTT retain after connecting to the configured trusted-LAN broker.

#### Scenario: USB-powered board boots with valid configuration
- **WHEN** the board joins Wi-Fi, reads averaged raw value 32768 and calculated percentage 50, and connects to the broker
- **THEN** it publishes `{"raw":32768,"percentage":50}` once with retain enabled and reports success through the serial console

#### Scenario: Continuous mode completes another measurement
- **WHEN** a later continuous-mode cycle obtains a new raw reading and percentage
- **THEN** the publisher replaces the retained topic value with a JSON object containing both new integers
