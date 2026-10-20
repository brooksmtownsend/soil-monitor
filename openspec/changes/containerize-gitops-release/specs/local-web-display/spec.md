# Spec Delta

## MODIFIED Requirements

### Requirement: HTTP lifecycle and diagnostics
When enabled, the HTTP server SHALL start and stop with the application context, shut down gracefully within a bounded interval, and surface unexpected listener or serving failures as application errors. When disabled, application lifecycle SHALL NOT require a web adapter.

#### Scenario: Enabled service is stopped
- **WHEN** the runtime sends the application its termination signal while HTTP is enabled
- **THEN** the HTTP listener stops accepting requests and active responses receive a bounded graceful-shutdown opportunity

#### Scenario: Listener cannot bind
- **WHEN** HTTP is enabled and the configured address is unavailable
- **THEN** the application exits non-zero with an HTTP listener diagnostic

#### Scenario: HTTP is disabled during shutdown
- **WHEN** the application receives a termination signal while HTTP is disabled
- **THEN** it shuts down normally without waiting for or closing a web listener

### Requirement: Read-only web interface
The web display SHALL expose no operation that changes moisture readings, runtime configuration, MQTT topics, or existing plant identifiers. Its state-changing operations SHALL be limited to registering a plant, assigning or replacing a display name, assigning or replacing a watering cadence, confirming watering, and uploading or replacing a plant photo.

#### Scenario: Unsupported root method is requested
- **WHEN** a client sends a non-read method to the root page
- **THEN** the server rejects the request without changing application state

#### Scenario: Unknown path is requested
- **WHEN** a client requests a path other than a documented overview, plant detail, registration, plant name, plant schedule, watering confirmation, or plant photo route
- **THEN** the server returns a not-found response

#### Scenario: Unsupported plant method is requested
- **WHEN** a client uses a method not documented for a plant detail, name, schedule, watering, or photo route
- **THEN** the server returns a method-not-allowed response without changing application state

### Requirement: Web display documentation
The project SHALL document HTTP enablement, the address setting, default URL, trusted-LAN mutation assumption, displayed fields, plant registration and naming, photo compatibility, watering schedules and confirmation, history charts, and verification procedure for the container deployment.

#### Scenario: Operator verifies the page
- **WHEN** an operator follows the deployment documentation with HTTP enabled
- **THEN** the operator can register a plant, upload a supported photo, manage its watering schedule, confirm watering, assign a plant name, and view stored moisture history from another LAN device

#### Scenario: Operator disables the page
- **WHEN** an operator follows the documented HTTP-disable procedure
- **THEN** MQTT collection and durable plant, care, photo, and moisture history data remain available without listening on the HTTP address

## ADDED Requirements

### Requirement: Hardware-free web display
The soil-monitor process SHALL optionally serve its web display from the same Go binary that handles MQTT, without requiring external web assets or an additional runtime service. The process SHALL NOT access GPIO, SPI, or other Raspberry Pi hardware.

#### Scenario: Web mode starts
- **WHEN** the process starts with valid MQTT and HTTP configuration
- **THEN** MQTT and the HTTP listener run within the same process without accessing hardware devices

#### Scenario: HTTP listener disabled
- **WHEN** the process starts with HTTP disabled
- **THEN** it does not create a network listener for the web display

## REMOVED Requirements

### Requirement: Single-binary HTTP display
**Reason**: The OLED HAT, its buttons, and plant selection are removed from the product.
**Migration**: Replaced by "Hardware-free web display".
