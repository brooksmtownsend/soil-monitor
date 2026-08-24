## ADDED Requirements

### Requirement: Layered runtime configuration
The application SHALL merge built-in defaults, an explicitly selected configuration file, process environment variables, and explicitly supplied command-line arguments in that ascending precedence order, then validate the resulting configuration before initializing adapters.

#### Scenario: Environment overrides the file
- **WHEN** an optional configuration file and the process environment provide different MQTT broker values and no broker argument is supplied
- **THEN** the application uses the process environment value

#### Scenario: Command line overrides the environment
- **WHEN** a command-line argument and the process environment provide different HTTP enablement values
- **THEN** the application uses the command-line value

#### Scenario: Required value is absent from every source
- **WHEN** the merged configuration has no MQTT broker or valid three-plant definition
- **THEN** the application exits before adapter initialization with a diagnostic identifying the missing or invalid setting

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

### Requirement: Complete command-line configuration surface
The application SHALL expose documented command-line arguments for the configuration-file path, MQTT broker and client identifier, three-plant definition, Pi HAT enablement and settings, and HTTP enablement and listen address. Boolean feature arguments SHALL accept explicit true or false values suitable for scripted invocation.

#### Scenario: Binary runs from direct inputs
- **WHEN** the operator supplies required values through process environment variables and uses command-line arguments to disable Pi HAT support
- **THEN** the single binary runs without a configuration file or Raspberry Pi hardware

#### Scenario: Argument value is invalid
- **WHEN** a command-line value cannot be parsed or fails configuration validation
- **THEN** the application exits with a diagnostic that identifies the argument or resulting setting

### Requirement: Backward-compatible defaults
Pi HAT support and the HTTP display SHALL default to enabled, and existing MQTT client, SPI port, debounce, and HTTP address defaults SHALL remain unchanged when their settings are omitted.

#### Scenario: Existing environment invocation is reused
- **WHEN** an existing valid `SOIL_MQTT_BROKER` and `SOIL_PLANTS` environment invocation supplies no new enablement settings or command-line arguments
- **THEN** the application initializes the Pi HAT and listens on the existing default HTTP address

### Requirement: Configuration help
The binary SHALL provide command-line help that documents each argument, its corresponding environment variable, defaults, configuration precedence, and an environment-only direct-run example.

#### Scenario: Operator requests help
- **WHEN** the operator invokes the binary with the help argument
- **THEN** it prints configuration and direct-run guidance without initializing hardware, MQTT, or HTTP
