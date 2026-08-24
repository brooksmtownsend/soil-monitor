## MODIFIED Requirements

### Requirement: Layered runtime configuration
The application SHALL merge built-in defaults, an explicitly selected configuration file, process environment variables, and explicitly supplied command-line arguments in that ascending precedence order, then validate the resulting configuration before initializing adapters. A valid MQTT broker and database path SHALL be required; a fixed plant definition SHALL NOT be required.

#### Scenario: Environment overrides the file
- **WHEN** an optional configuration file and the process environment provide different database paths and no database argument is supplied
- **THEN** the application uses the process environment value

#### Scenario: Command line overrides the environment
- **WHEN** a command-line argument and the process environment provide different HTTP enablement values
- **THEN** the application uses the command-line value

#### Scenario: Required value is absent from every source
- **WHEN** the merged configuration has no valid MQTT broker or database path
- **THEN** the application exits before adapter initialization with a diagnostic identifying the missing or invalid setting

#### Scenario: No plants are configured
- **WHEN** the merged configuration contains no plant definition
- **THEN** configuration validation succeeds and plants can be discovered from MQTT

### Requirement: Complete command-line configuration surface
The application SHALL expose documented command-line arguments for the configuration-file path, MQTT broker and client identifier, database path, Pi HAT enablement and settings, and HTTP enablement and listen address. It SHALL NOT require a plant-definition argument. Boolean feature arguments SHALL accept explicit true or false values suitable for scripted invocation.

#### Scenario: Binary runs from direct inputs
- **WHEN** the operator supplies required broker and database values through process environment variables and uses command-line arguments to disable Pi HAT support
- **THEN** the single binary runs without a configuration file, predefined plants, or Raspberry Pi hardware

#### Scenario: Argument value is invalid
- **WHEN** a command-line value cannot be parsed or fails configuration validation
- **THEN** the application exits with a diagnostic that identifies the argument or resulting setting

### Requirement: Backward-compatible defaults
Pi HAT support and the HTTP display SHALL default to enabled, the database path SHALL default to `/var/lib/soil-monitor/soil-monitor.db`, and existing MQTT client, SPI port, debounce, and HTTP address defaults SHALL remain unchanged when their settings are omitted.

#### Scenario: Minimal environment invocation is used
- **WHEN** a valid `SOIL_MQTT_BROKER` is supplied with no feature, database, or plant settings
- **THEN** the application initializes the Pi HAT, listens on the existing default HTTP address, and uses the default database path

### Requirement: Configuration help
The binary SHALL provide command-line help that documents each argument, its corresponding environment variable, defaults, configuration precedence, database persistence, and an environment-only direct-run example that does not require predefined plants.

#### Scenario: Operator requests help
- **WHEN** the operator invokes the binary with the help argument
- **THEN** it prints configuration and direct-run guidance without initializing the database, hardware, MQTT, or HTTP

## ADDED Requirements

### Requirement: Configurable database path
The application SHALL independently configure `SOIL_DATABASE_PATH` / `--database-path`, SHALL default it to `/var/lib/soil-monitor/soil-monitor.db`, and SHALL reject an empty path before adapter initialization.

#### Scenario: Database path is overridden
- **WHEN** the merged configuration supplies a non-empty database path
- **THEN** the application opens or creates its SQLite database at that path

#### Scenario: Database path is empty
- **WHEN** the highest-precedence database path value is empty
- **THEN** startup exits non-zero with a database-path configuration diagnostic

