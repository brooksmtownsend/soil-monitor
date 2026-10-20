# plant-moisture-display Specification

## Purpose

Define dynamic plant discovery, MQTT reading validation, durable ingestion, and process lifecycle behavior. The OLED display and button navigation formerly described here have been removed.

## Requirements

### Requirement: Moisture topic subscription
The application SHALL subscribe to `plants/+/moisture`, interpret the wildcard topic segment as a plant identifier, and dynamically discover valid identifiers without a preconfigured allowlist. A reading whose identifier belongs to a manually registered plant SHALL update that existing plant rather than attempting to create another catalog entry.

#### Scenario: Application connects to MQTT
- **WHEN** an MQTT connection is established
- **THEN** the application subscribes to `plants/+/moisture`

#### Scenario: Application reconnects to MQTT
- **WHEN** a lost MQTT connection is re-established
- **THEN** the application restores its `plants/+/moisture` subscription without operator action

#### Scenario: New identifier publishes a reading
- **WHEN** a valid message arrives on `plants/money-tree/moisture` and `money-tree` is not yet known
- **THEN** the application discovers and stores `money-tree` without requiring configuration changes

#### Scenario: Registered identifier publishes its first reading
- **WHEN** `money-tree` was registered through the web interface without a reading and a valid message arrives on `plants/money-tree/moisture`
- **THEN** the application appends the reading to that plant and preserves its display name, photo, and watering state

### Requirement: Invalid messages preserve current state
The application SHALL ignore messages with an invalid topic or plant identifier, malformed or trailing JSON, missing or non-integer required fields, negative raw values, or percentages outside 0 through 100. Ignored messages MUST NOT create a plant, append history, replace a previously accepted value, or terminate the application.

#### Scenario: Legacy integer payload is received
- **WHEN** payload `47` arrives on a moisture topic
- **THEN** the application logs the rejected message and preserves durable and in-memory state

#### Scenario: Required field is invalid
- **WHEN** a payload omits `raw` or `percentage` or supplies either required field as a non-integer
- **THEN** the application logs the rejected message and preserves durable and in-memory state

#### Scenario: Value is out of range
- **WHEN** a payload contains a negative raw value or a percentage below 0 or above 100
- **THEN** the application logs the rejected message and preserves durable and in-memory state

#### Scenario: Plant identifier is invalid
- **WHEN** an otherwise valid JSON reading arrives on a topic whose wildcard segment is not a kebab-case plant identifier
- **THEN** the application does not discover the plant and preserves durable and in-memory state

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
