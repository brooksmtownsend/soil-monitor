# Spec Delta

## ADDED Requirements

### Requirement: Startup configuration
The application SHALL require a valid MQTT broker URL and local database path and SHALL reject invalid configuration before initializing normal operation. It SHALL NOT require plant identifiers or names at startup.

#### Scenario: Empty catalog starts
- **WHEN** the application starts with a broker URL and an initialized empty database
- **THEN** it initializes MQTT state handling without requiring a configured plant

#### Scenario: Invalid required configuration
- **WHEN** the application starts with an invalid broker URL or database path
- **THEN** it exits with an error that identifies the invalid configuration

### Requirement: Valid moisture ingestion
The application SHALL accept a JSON object containing integer `raw` and `percentage` fields for a valid plant topic when raw is non-negative and percentage is from 0 through 100 inclusive. It SHALL durably append the reading and retain that plant's latest accepted raw value, percentage, and receipt timestamp in memory.

#### Scenario: Plant receives a valid reading
- **WHEN** payload `{"raw":32768,"percentage":47}` arrives on `plants/money-tree/moisture`
- **THEN** the application stores both values as that plant's latest reading

#### Scenario: Payload contains an additional field
- **WHEN** an otherwise valid moisture JSON object contains an unrecognized field
- **THEN** the application accepts the required raw and percentage values for forward compatibility

### Requirement: Process lifecycle
The application SHALL fail startup with a useful error if an enabled adapter cannot be initialized. After successful startup, it SHALL handle a normal termination signal by disconnecting MQTT, closing the history repository, and releasing every enabled server resource.

#### Scenario: Database cannot be initialized
- **WHEN** the history database cannot be opened at startup
- **THEN** the application exits non-zero with an error identifying history initialization as the cause

#### Scenario: Process is stopped
- **WHEN** the running application receives a normal termination signal
- **THEN** it releases MQTT, database, and server resources and exits without requiring a forced kill

### Requirement: Container test documentation
The project SHALL document how to configure and run the application as a container, how web registration and dynamic MQTT discovery coexist, how plant naming and watering schedules work, and how to publish JSON test readings from another computer.

#### Scenario: Operator performs a basic MQTT test
- **WHEN** an operator follows the documented setup and publishes a valid JSON reading to a new or registered plant topic
- **THEN** the plant and value appear on the web dashboard without editing plant configuration

#### Scenario: Operator registers a plant before publishing
- **WHEN** an operator follows the documented web workflow to register a plant and later publishes to its derived identifier
- **THEN** the existing plant gains the reading without losing its care metadata

## REMOVED Requirements

### Requirement: Valid startup configuration
**Reason**: Pi HAT enablement and hardware settings are removed.
**Migration**: Replaced by "Startup configuration".

### Requirement: Valid moisture updates
**Reason**: Scenarios referred to the OLED-selected plant.
**Migration**: Replaced by "Valid moisture ingestion".

### Requirement: Selected plant display
**Reason**: The OLED HAT, its buttons, and plant selection are removed from the product.
**Migration**: View plants and readings on the web dashboard.

### Requirement: Button navigation
**Reason**: The OLED HAT, its buttons, and plant selection are removed from the product.
**Migration**: None; use the web dashboard.

### Requirement: MQTT connection status
**Reason**: The OLED HAT, its buttons, and plant selection are removed from the product. Connection state remains shown by the web dashboard (local-web-display).
**Migration**: Use the web dashboard.

### Requirement: Graceful process lifecycle
**Reason**: Scenarios referred to display and input hardware.
**Migration**: Replaced by "Process lifecycle".

### Requirement: Bench-test documentation
**Reason**: It documented Raspberry Pi and Debian host modes.
**Migration**: Replaced by "Container test documentation".
