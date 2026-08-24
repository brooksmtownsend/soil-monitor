## MODIFIED Requirements

### Requirement: Valid startup configuration
The application SHALL require an MQTT broker URL and exactly three plants, each with a unique identifier and a non-empty display name. It SHALL independently configure whether Pi HAT hardware is enabled and SHALL reject invalid configuration before initializing normal operation. Hardware-specific settings SHALL be required and validated only when Pi HAT support is enabled.

#### Scenario: Three valid plants and Pi HAT are configured
- **WHEN** the application starts with a broker URL, three uniquely identified plants, and Pi HAT support enabled
- **THEN** it initializes the hardware and MQTT connection using that configuration

#### Scenario: Three valid plants are configured without Pi HAT
- **WHEN** the application starts with a broker URL, three uniquely identified plants, and Pi HAT support disabled
- **THEN** it initializes MQTT state handling without initializing GPIO, SPI, display, or buttons

#### Scenario: Plant configuration is invalid
- **WHEN** the application starts with other than three plants, a blank field, or duplicate plant identifiers
- **THEN** it exits with an error that identifies the invalid configuration

#### Scenario: Disabled hardware setting is absent
- **WHEN** Pi HAT support is disabled and no SPI port or button debounce value is supplied
- **THEN** hardware settings do not prevent startup

### Requirement: Selected plant display
When Pi HAT support is enabled, the OLED SHALL show the selected plant's display name, its latest moisture percentage or a no-reading placeholder, and its one-based position out of three. The application SHALL initially select the first configured plant. When Pi HAT support is disabled, MQTT updates SHALL continue without an OLED render attempt.

#### Scenario: Application starts before readings arrive
- **WHEN** the display is enabled and initialized and no value has been accepted for the first configured plant
- **THEN** the OLED shows that plant's name, a no-reading placeholder, and `1 / 3`

#### Scenario: Selected plant has a reading
- **WHEN** the display is enabled and the selected plant has a latest accepted value of 72
- **THEN** the OLED shows the plant name, `72%`, and the plant's position out of three

#### Scenario: Plant name exceeds the available width
- **WHEN** the display is enabled and a configured display name cannot fit in its allocated OLED region
- **THEN** the renderer truncates it deterministically without obscuring the moisture value or position

#### Scenario: HAT is disabled
- **WHEN** a valid MQTT reading arrives while Pi HAT support is disabled
- **THEN** the application stores the reading without attempting to render an OLED frame

### Requirement: Button navigation
When Pi HAT support is enabled, the application SHALL map BCM GPIO 21 to previous-plant navigation, BCM GPIO 20 to next-plant navigation, and BCM GPIO 16 to a reserved no-op action. Previous and next navigation SHALL wrap around the three configured plants, and a physical press SHALL be debounced into at most one navigation action within the debounce interval. When Pi HAT support is disabled, the application SHALL NOT initialize or await GPIO button events.

#### Scenario: Next is pressed on the first plant
- **WHEN** GPIO 20 registers a debounced press while Pi HAT support is enabled and plant 1 is selected
- **THEN** plant 2 becomes selected and the display refreshes with plant 2's state

#### Scenario: Previous wraps from the first plant
- **WHEN** GPIO 21 registers a debounced press while Pi HAT support is enabled and plant 1 is selected
- **THEN** plant 3 becomes selected and the display refreshes with plant 3's state

#### Scenario: Next wraps from the last plant
- **WHEN** GPIO 20 registers a debounced press while Pi HAT support is enabled and plant 3 is selected
- **THEN** plant 1 becomes selected and the display refreshes with plant 1's state

#### Scenario: Reserved button is pressed
- **WHEN** GPIO 16 registers a debounced press while Pi HAT support is enabled
- **THEN** the selected plant and displayed view remain unchanged

#### Scenario: HAT is disabled
- **WHEN** the application starts with Pi HAT support disabled
- **THEN** it does not access the configured button GPIO pins

### Requirement: MQTT connection status
When Pi HAT support is enabled, the application SHALL indicate on the OLED when MQTT is disconnected while continuing to show the selected plant's last accepted value and SHALL clear the indication after reconnection. When Pi HAT support is disabled, connection transitions SHALL continue updating shared state without an OLED render attempt.

#### Scenario: MQTT connection is lost after a reading
- **WHEN** the broker connection is lost after the selected plant has received a valid value and Pi HAT support is enabled
- **THEN** the OLED continues showing that value and adds a disconnected indication

#### Scenario: MQTT connection is restored
- **WHEN** the application reconnects to the broker and Pi HAT support is enabled
- **THEN** the OLED clears the disconnected indication and preserves all in-memory values

#### Scenario: Connection changes without HAT
- **WHEN** MQTT disconnects or reconnects while Pi HAT support is disabled
- **THEN** shared connection state changes without accessing display hardware

### Requirement: Graceful process lifecycle
The application SHALL fail startup with a useful error if an enabled adapter cannot be initialized. After successful startup, it SHALL handle a normal termination signal by disconnecting MQTT and releasing every enabled display, input, and server resource without requiring disabled adapters to exist.

#### Scenario: Enabled display initialization fails
- **WHEN** Pi HAT support is enabled and the OLED cannot be initialized at startup
- **THEN** the application exits non-zero with an error identifying display initialization as the cause

#### Scenario: Disabled display is unavailable
- **WHEN** Pi HAT support is disabled on a host without GPIO or SPI devices
- **THEN** missing display hardware does not prevent startup or shutdown

#### Scenario: Service is stopped
- **WHEN** the running application receives a normal termination signal
- **THEN** it releases MQTT and all enabled display, input, and server resources and exits without requiring a forced kill

### Requirement: Bench-test documentation
The project SHALL document how to configure and run the application on Raspberry Pi OS or a generic Debian host, how to select Pi HAT and HTTP modes, and how to publish test values for each configured plant from another computer.

#### Scenario: Operator performs a basic MQTT test
- **WHEN** an operator follows the documented setup for either supported host mode and publishes a valid value to one configured plant topic
- **THEN** the value appears on every enabled display surface
