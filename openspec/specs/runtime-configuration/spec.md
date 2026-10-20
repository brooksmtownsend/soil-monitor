# runtime-configuration Specification

## Purpose

Define layered file, environment, and command-line configuration for database, MQTT, and HTTP operation.

## Requirements

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

### Requirement: Optional non-executable configuration file
The application SHALL accept an explicitly named configuration file containing the documented `KEY=VALUE` subset, SHALL support blank lines, comments, and documented quoted values, and MUST NOT evaluate shell expansion or commands while reading it. The application SHALL NOT require or implicitly load a file when no file argument is provided.

#### Scenario: Explicit file is valid
- **WHEN** the operator starts the application with `--config` naming a readable valid file
- **THEN** the file values participate in layered configuration

#### Scenario: No file is selected
- **WHEN** required settings are supplied through direct process environment variables and no `--config` argument is present
- **THEN** the application starts without first creating or reading a configuration file

#### Scenario: Explicit file is unavailable
- **WHEN** `--config` names a missing or unreadable file
- **THEN** the application exits with a configuration-file diagnostic before adapter initialization

#### Scenario: File contains executable shell syntax
- **WHEN** a selected file contains unsupported expansion or command syntax
- **THEN** the application rejects the file without executing that syntax

### Requirement: Configuration help
The binary SHALL provide command-line help that documents each argument, its corresponding environment variable, defaults, configuration precedence, database persistence, and an environment-only direct-run example that does not require predefined plants.

#### Scenario: Operator requests help
- **WHEN** the operator invokes the binary with the help argument
- **THEN** it prints configuration and direct-run guidance without initializing the database, hardware, MQTT, or HTTP

### Requirement: Configurable database path
The application SHALL independently configure `SOIL_DATABASE_PATH` / `--database-path`, SHALL default it to `/var/lib/soil-monitor/soil-monitor.db`, and SHALL reject an empty path before adapter initialization.

#### Scenario: Database path is overridden
- **WHEN** the merged configuration supplies a non-empty database path
- **THEN** the application opens or creates its SQLite database at that path

#### Scenario: Database path is empty
- **WHEN** the highest-precedence database path value is empty
- **THEN** startup exits non-zero with a database-path configuration diagnostic

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
