## ADDED Requirements

### Requirement: Valid startup configuration
The application SHALL require an MQTT broker URL and exactly three plants, each with a unique identifier and a non-empty display name. The application SHALL reject invalid or duplicate plant configuration before initializing normal operation.

#### Scenario: Three valid plants are configured
- **WHEN** the application starts with a broker URL and three uniquely identified plants
- **THEN** it initializes the hardware and MQTT connection using that configuration

#### Scenario: Plant configuration is invalid
- **WHEN** the application starts with other than three plants, a blank field, or duplicate plant identifiers
- **THEN** it exits with an error that identifies the invalid configuration

### Requirement: Moisture topic subscription
The application SHALL subscribe to `plants/+/moisture` and interpret the wildcard topic segment as a configured plant identifier.

#### Scenario: Application connects to MQTT
- **WHEN** an MQTT connection is established
- **THEN** the application subscribes to `plants/+/moisture`

#### Scenario: Application reconnects to MQTT
- **WHEN** a lost MQTT connection is re-established
- **THEN** the application restores its `plants/+/moisture` subscription without operator action

### Requirement: Valid moisture updates
The application SHALL accept a base-10 integer payload from 0 through 100 inclusive for a configured plant and SHALL retain that plant's latest accepted value in memory.

#### Scenario: Selected plant receives a valid value
- **WHEN** a message with payload `47` arrives on `plants/money-tree/moisture` and `money-tree` is configured and selected
- **THEN** the application stores 47 for that plant and refreshes the display to show `47%`

#### Scenario: Non-selected plant receives a valid value
- **WHEN** a valid moisture message arrives for a configured plant that is not selected
- **THEN** the application stores the value without changing the selected plant

### Requirement: Invalid messages preserve current state
The application SHALL ignore messages for unknown plant identifiers, non-integer payloads, and integer payloads outside 0 through 100. Ignored messages MUST NOT replace any previously accepted value or terminate the application.

#### Scenario: Payload is malformed
- **WHEN** a configured plant receives a payload that is not a base-10 integer
- **THEN** the application logs the rejected message and preserves the plant's previous value

#### Scenario: Payload is out of range
- **WHEN** a configured plant receives an integer below 0 or above 100
- **THEN** the application logs the rejected message and preserves the plant's previous value

#### Scenario: Plant identifier is unknown
- **WHEN** a valid integer arrives on a moisture topic for an unconfigured plant identifier
- **THEN** the application logs or records that the message was ignored and leaves all configured plant values unchanged

### Requirement: Selected plant display
The OLED SHALL show the selected plant's display name, its latest moisture percentage or a no-reading placeholder, and its one-based position out of three. The application SHALL initially select the first configured plant.

#### Scenario: Application starts before readings arrive
- **WHEN** the display is initialized and no value has been accepted for the first configured plant
- **THEN** the OLED shows that plant's name, a no-reading placeholder, and `1 / 3`

#### Scenario: Selected plant has a reading
- **WHEN** the selected plant has a latest accepted value of 72
- **THEN** the OLED shows the plant name, `72%`, and the plant's position out of three

#### Scenario: Plant name exceeds the available width
- **WHEN** a configured display name cannot fit in its allocated OLED region
- **THEN** the renderer truncates it deterministically without obscuring the moisture value or position

### Requirement: Button navigation
The application SHALL map BCM GPIO 21 to previous-plant navigation, BCM GPIO 20 to next-plant navigation, and BCM GPIO 16 to a reserved no-op action. Previous and next navigation SHALL wrap around the three configured plants, and a physical press SHALL be debounced into at most one navigation action within the debounce interval.

#### Scenario: Next is pressed on the first plant
- **WHEN** GPIO 20 registers a debounced press while plant 1 is selected
- **THEN** plant 2 becomes selected and the display refreshes with plant 2's state

#### Scenario: Previous wraps from the first plant
- **WHEN** GPIO 21 registers a debounced press while plant 1 is selected
- **THEN** plant 3 becomes selected and the display refreshes with plant 3's state

#### Scenario: Next wraps from the last plant
- **WHEN** GPIO 20 registers a debounced press while plant 3 is selected
- **THEN** plant 1 becomes selected and the display refreshes with plant 1's state

#### Scenario: Reserved button is pressed
- **WHEN** GPIO 16 registers a debounced press
- **THEN** the selected plant and displayed view remain unchanged

### Requirement: MQTT connection status
The application SHALL indicate on the OLED when MQTT is disconnected while continuing to show the selected plant's last accepted value. The application SHALL clear the disconnected indication after reconnection.

#### Scenario: MQTT connection is lost after a reading
- **WHEN** the broker connection is lost after the selected plant has received a valid value
- **THEN** the OLED continues showing that value and adds a disconnected indication

#### Scenario: MQTT connection is restored
- **WHEN** the application reconnects to the broker
- **THEN** the OLED clears the disconnected indication and preserves all in-memory values

### Requirement: Graceful process lifecycle
The application SHALL fail startup with a useful error if required display or GPIO hardware cannot be initialized. After successful startup, the application SHALL handle a normal termination signal by stopping input watchers, disconnecting MQTT, and releasing display and hardware resources.

#### Scenario: Display initialization fails
- **WHEN** the OLED cannot be initialized at startup
- **THEN** the application exits non-zero with an error identifying display initialization as the cause

#### Scenario: Service is stopped
- **WHEN** the running application receives a normal termination signal
- **THEN** it releases its MQTT, display, and GPIO resources and exits without requiring a forced kill

### Requirement: Bench-test documentation
The project SHALL document how to configure and run the application on Debian Lite and how to publish test values for each configured plant from another computer.

#### Scenario: Operator performs a basic MQTT test
- **WHEN** an operator follows the documented setup and publishes a valid value to one configured plant topic
- **THEN** the corresponding value appears when that plant is selected on the OLED
