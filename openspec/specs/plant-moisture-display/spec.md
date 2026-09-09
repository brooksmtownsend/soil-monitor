# plant-moisture-display Specification

## Purpose

Define dynamic plant discovery, MQTT reading validation, OLED presentation, navigation, connection status, and process lifecycle behavior.

## Requirements

### Requirement: Valid startup configuration
The application SHALL require a valid MQTT broker URL and local database path, SHALL independently configure whether Pi HAT hardware is enabled, and SHALL reject invalid configuration before initializing normal operation. It SHALL NOT require plant identifiers or names at startup. Hardware-specific settings SHALL be required and validated only when Pi HAT support is enabled.

#### Scenario: Empty catalog and Pi HAT are configured
- **WHEN** the application starts with a broker URL, an initialized empty database, and Pi HAT support enabled
- **THEN** it initializes hardware and MQTT without requiring a configured plant

#### Scenario: Empty catalog starts without Pi HAT
- **WHEN** the application starts with a broker URL, an initialized empty database, and Pi HAT support disabled
- **THEN** it initializes MQTT state handling without initializing GPIO, SPI, display, or buttons

#### Scenario: Required configuration is invalid
- **WHEN** the application starts with an invalid broker URL or database path
- **THEN** it exits with an error that identifies the invalid configuration

#### Scenario: Disabled hardware setting is absent
- **WHEN** Pi HAT support is disabled and no SPI port or button debounce value is supplied
- **THEN** hardware settings do not prevent startup

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

### Requirement: Valid moisture updates
The application SHALL accept a JSON object containing integer `raw` and `percentage` fields for a valid plant topic when raw is non-negative and percentage is from 0 through 100 inclusive. It SHALL durably append the reading and retain that plant's latest accepted raw value, percentage, and receipt timestamp in memory.

#### Scenario: Selected plant receives a valid reading
- **WHEN** payload `{"raw":32768,"percentage":47}` arrives on `plants/money-tree/moisture` and `money-tree` is selected
- **THEN** the application stores both values and refreshes the display to show `47%`

#### Scenario: Non-selected plant receives a valid reading
- **WHEN** a valid moisture message arrives for a discovered plant that is not selected
- **THEN** the application stores the reading without changing the selected plant

#### Scenario: Payload contains an additional field
- **WHEN** an otherwise valid moisture JSON object contains an unrecognized field
- **THEN** the application accepts the required raw and percentage values for forward compatibility

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

### Requirement: Selected plant display
When Pi HAT support is enabled, the OLED SHALL show the selected cataloged plant's assigned name or identifier, its latest moisture percentage or a no-reading placeholder, and its one-based position out of the current plant count. Restored plants SHALL be ordered by first-seen time with identifier as a deterministic tie-breaker, and a newly registered or MQTT-discovered plant SHALL be appended without changing an existing selection. When no plants exist, the OLED SHALL show an explicit waiting-for-plants state. When Pi HAT support is disabled, MQTT and web updates SHALL continue without an OLED render attempt.

#### Scenario: Application starts with an empty catalog
- **WHEN** the display is enabled and no plant exists
- **THEN** the OLED shows a waiting-for-plants state without a fabricated plant or position

#### Scenario: First plant is registered
- **WHEN** a valid web registration creates the first plant while the display is enabled
- **THEN** that plant becomes selected and the OLED shows its label, no-reading placeholder, and `1 / 1`

#### Scenario: First plant is discovered
- **WHEN** a valid reading discovers the first plant while the display is enabled
- **THEN** that plant becomes selected and the OLED shows its label, percentage, and `1 / 1`

#### Scenario: Selected plant has a reading
- **WHEN** the selected plant has a latest accepted percentage of 72 among five cataloged plants
- **THEN** the OLED shows the plant label, `72%`, and its one-based position out of five

#### Scenario: Plant name exceeds the available width
- **WHEN** the display is enabled and an assigned display name cannot fit in its allocated OLED region
- **THEN** the renderer truncates it deterministically without obscuring the moisture value or position

#### Scenario: HAT is disabled
- **WHEN** a valid MQTT reading or web registration arrives while Pi HAT support is disabled
- **THEN** the application stores the update without attempting to render an OLED frame

### Requirement: Button navigation
When Pi HAT support is enabled, the application SHALL map BCM GPIO 21 to previous-plant navigation, BCM GPIO 20 to next-plant navigation, and BCM GPIO 16 to a reserved no-op action. Previous and next navigation SHALL wrap around the current plant catalog, a press when the collection is empty SHALL be a no-op, and a physical press SHALL be debounced into at most one navigation action within the debounce interval. When Pi HAT support is disabled, the application SHALL NOT initialize or await GPIO button events.

#### Scenario: Next is pressed with multiple plants
- **WHEN** GPIO 20 registers a debounced press while Pi HAT support is enabled and a non-final plant is selected
- **THEN** the following cataloged plant becomes selected and the display refreshes with its state

#### Scenario: Previous wraps from the first plant
- **WHEN** GPIO 21 registers a debounced press while Pi HAT support is enabled and the first plant is selected
- **THEN** the last plant becomes selected

#### Scenario: Next wraps from the last plant
- **WHEN** GPIO 20 registers a debounced press while Pi HAT support is enabled and the last plant is selected
- **THEN** the first plant becomes selected

#### Scenario: Navigation is pressed with no plants
- **WHEN** a previous or next press occurs while the catalog is empty
- **THEN** the selection and waiting-for-plants display remain unchanged

#### Scenario: Reserved button is pressed
- **WHEN** GPIO 16 registers a debounced press while Pi HAT support is enabled
- **THEN** the selected plant and displayed view remain unchanged

#### Scenario: HAT is disabled
- **WHEN** the application starts with Pi HAT support disabled
- **THEN** it does not access the configured button GPIO pins

### Requirement: MQTT connection status
When Pi HAT support is enabled, the application SHALL indicate on the OLED when MQTT is disconnected while continuing to show the selected plant's last accepted value and SHALL clear the indication after reconnection. When Pi HAT support is disabled, connection transitions SHALL continue updating shared state without an OLED render attempt.

#### Scenario: MQTT connection is lost after a reading
- **WHEN** the MQTT connection is lost after the selected plant has received a valid value and Pi HAT support is enabled
- **THEN** the OLED continues showing that value and adds a disconnected indication

#### Scenario: MQTT connection is restored
- **WHEN** the application reconnects to the broker and Pi HAT support is enabled
- **THEN** the OLED clears the disconnected indication and preserves all in-memory values

#### Scenario: Connection changes without HAT
- **WHEN** MQTT disconnects or reconnects while Pi HAT support is disabled
- **THEN** shared connection state changes without accessing display hardware

### Requirement: Graceful process lifecycle
The application SHALL fail startup with a useful error if an enabled adapter cannot be initialized. After successful startup, it SHALL handle a normal termination signal by disconnecting MQTT, closing the history repository, and releasing every enabled display, input, and server resource without requiring disabled adapters to exist.

#### Scenario: Enabled display initialization fails
- **WHEN** Pi HAT support is enabled and the OLED cannot be initialized at startup
- **THEN** the application exits non-zero with an error identifying display initialization as the cause

#### Scenario: Disabled display is unavailable
- **WHEN** Pi HAT support is disabled on a host without GPIO or SPI devices
- **THEN** missing display hardware does not prevent startup or shutdown

#### Scenario: Service is stopped
- **WHEN** the running application receives a normal termination signal
- **THEN** it releases MQTT, database, and all enabled display, input, and server resources and exits without requiring a forced kill

### Requirement: Bench-test documentation
The project SHALL document how to configure and run the application on Raspberry Pi OS or a generic Debian host, how to select Pi HAT and HTTP modes, how web registration and dynamic MQTT discovery coexist, how plant naming and watering schedules work, and how to publish JSON test readings from another computer.

#### Scenario: Operator performs a basic MQTT test
- **WHEN** an operator follows the documented setup for either supported host mode and publishes a valid JSON reading to a new or registered plant topic
- **THEN** the plant and value appear on every enabled display surface without editing plant configuration

#### Scenario: Operator registers a plant before its sensor publishes
- **WHEN** an operator follows the documented web workflow to register a plant and later publishes to its derived identifier
- **THEN** the existing plant gains the reading without losing its care metadata
