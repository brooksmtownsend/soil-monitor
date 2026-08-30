## MODIFIED Requirements

### Requirement: Configurable HTTP listener
The application SHALL independently configure HTTP enablement and `SOIL_HTTP_ADDR`. HTTP SHALL default to enabled, the address SHALL default to `:80`, and a malformed address SHALL be rejected only when HTTP is enabled.

#### Scenario: Default address is used
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` is unset or blank
- **THEN** the HTTP server listens on port 80 on the configured host interfaces

#### Scenario: Address is overridden
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` contains a valid host-and-port listen address
- **THEN** the HTTP server uses the configured address

#### Scenario: Enabled address is invalid
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` does not contain a valid listen port
- **THEN** startup exits non-zero with a configuration diagnostic before starting the application

#### Scenario: Disabled address is absent
- **WHEN** HTTP is disabled and no listen address is supplied
- **THEN** HTTP address validation does not prevent startup
