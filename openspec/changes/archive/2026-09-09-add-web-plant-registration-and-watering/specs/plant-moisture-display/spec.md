## MODIFIED Requirements

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

### Requirement: Bench-test documentation
The project SHALL document how to configure and run the application on Raspberry Pi OS or a generic Debian host, how to select Pi HAT and HTTP modes, how web registration and dynamic MQTT discovery coexist, how plant naming and watering schedules work, and how to publish JSON test readings from another computer.

#### Scenario: Operator performs a basic MQTT test
- **WHEN** an operator follows the documented setup for either supported host mode and publishes a valid JSON reading to a new or registered plant topic
- **THEN** the plant and value appear on every enabled display surface without editing plant configuration

#### Scenario: Operator registers a plant before its sensor publishes
- **WHEN** an operator follows the documented web workflow to register a plant and later publishes to its derived identifier
- **THEN** the existing plant gains the reading without losing its care metadata
