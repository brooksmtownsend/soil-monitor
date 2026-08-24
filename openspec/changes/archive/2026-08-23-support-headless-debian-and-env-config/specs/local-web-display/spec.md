## MODIFIED Requirements

### Requirement: Single-binary HTTP display
The soil-monitor process SHALL optionally serve its web display from the same Go binary that handles MQTT and any enabled OLED and button adapters, without requiring external web assets or an additional runtime service. HTTP enablement SHALL be independent of Pi HAT enablement.

#### Scenario: All displays are enabled
- **WHEN** the process starts with valid Pi HAT, MQTT, and HTTP configuration
- **THEN** the OLED workflow and HTTP listener run within the same process

#### Scenario: Headless web mode starts
- **WHEN** the process starts with Pi HAT support disabled and HTTP enabled
- **THEN** MQTT and the HTTP listener run without accessing Pi hardware

#### Scenario: HTTP is disabled
- **WHEN** the process starts with HTTP disabled
- **THEN** it does not create a network listener for the web display

### Requirement: Configurable HTTP listener
The application SHALL independently configure HTTP enablement and `SOIL_HTTP_ADDR`. HTTP SHALL default to enabled, the address SHALL default to `:8080`, and a malformed address SHALL be rejected only when HTTP is enabled.

#### Scenario: Default address is used
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` is unset or blank
- **THEN** the HTTP server listens on port 8080 on the configured host interfaces

#### Scenario: Address is overridden
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` contains a valid host-and-port listen address
- **THEN** the HTTP server uses the configured address

#### Scenario: Enabled address is invalid
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` does not contain a valid listen port
- **THEN** startup exits non-zero with a configuration diagnostic before starting the application

#### Scenario: Disabled address is absent
- **WHEN** HTTP is disabled and no listen address is supplied
- **THEN** HTTP address validation does not prevent startup

### Requirement: HTTP lifecycle and diagnostics
When enabled, the HTTP server SHALL start and stop with the application context, shut down gracefully within a bounded interval, and surface unexpected listener or serving failures as application errors. When disabled, application lifecycle SHALL NOT require a web adapter.

#### Scenario: Enabled service is stopped
- **WHEN** systemd sends the application its termination signal while HTTP is enabled
- **THEN** the HTTP listener stops accepting requests and active responses receive a bounded graceful-shutdown opportunity

#### Scenario: Listener cannot bind
- **WHEN** HTTP is enabled and the configured address is unavailable
- **THEN** the application exits non-zero with an HTTP listener diagnostic

#### Scenario: HTTP is disabled during shutdown
- **WHEN** the application receives a termination signal while HTTP is disabled
- **THEN** it shuts down normally without waiting for or closing a web listener

### Requirement: Web display documentation
The project SHALL document HTTP enablement, the address setting, default URL, trusted-LAN assumption, displayed fields, and verification procedure for both Pi and headless Debian deployments.

#### Scenario: Operator verifies the page
- **WHEN** an operator follows the deployment documentation with HTTP enabled
- **THEN** the operator can configure the listener, run or release the binary, and open the status page from another LAN device

#### Scenario: Operator disables the page
- **WHEN** an operator follows the documented HTTP-disable procedure
- **THEN** the soil-monitor process continues its other enabled functions without listening on the HTTP address
