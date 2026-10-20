# Spec Delta

## ADDED Requirements

### Requirement: Command-line configuration surface
The application SHALL expose documented command-line arguments for the configuration-file path, MQTT broker and client identifier, database path, and HTTP enablement and listen address. It SHALL NOT require a plant-definition argument and SHALL NOT accept Pi HAT, SPI, or button settings. Boolean feature arguments SHALL accept explicit true or false values suitable for scripted invocation.

#### Scenario: Binary runs from direct inputs
- **WHEN** the operator supplies required broker and database values through process environment variables
- **THEN** the single binary runs without a configuration file or predefined plants

#### Scenario: Command-line value is invalid
- **WHEN** a command-line value cannot be parsed or fails configuration validation
- **THEN** the application exits with a diagnostic that identifies the argument or resulting setting

#### Scenario: Removed hardware setting is supplied
- **WHEN** the operator supplies a removed Pi HAT, SPI, or button argument or configuration-file key
- **THEN** the application exits with a diagnostic identifying the unrecognized setting

### Requirement: Default settings
The HTTP display SHALL default to enabled, the database path SHALL default to `/var/lib/soil-monitor/soil-monitor.db`, and the MQTT client and HTTP address defaults SHALL remain unchanged when their settings are omitted.

#### Scenario: Minimal environment invocation
- **WHEN** a valid `SOIL_MQTT_BROKER` is supplied with no feature, database, or plant settings
- **THEN** the application listens on the existing default HTTP address and uses the default database path

## REMOVED Requirements

### Requirement: Complete command-line configuration surface
**Reason**: Pi HAT settings are removed.
**Migration**: Replaced by "Command-line configuration surface".

### Requirement: Backward-compatible defaults
**Reason**: Pi HAT, SPI, and debounce defaults are removed.
**Migration**: Replaced by "Default settings".
