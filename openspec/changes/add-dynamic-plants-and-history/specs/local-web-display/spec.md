## MODIFIED Requirements

### Requirement: Plant status page
The root page SHALL render valid HTML containing every discovered plant's assigned display name or identifier, latest raw reading and moisture percentage, reading time, and the current MQTT connection state. Each plant SHALL link to its detail page.

#### Scenario: All readings are known
- **WHEN** accepted MQTT readings exist for all discovered plants
- **THEN** `GET /` returns status 200 and shows each plant with its latest raw value, percentage, and reading time

#### Scenario: Restored plant has no reading
- **WHEN** a stored plant has no historical reading
- **THEN** the page shows that plant with explicit unknown placeholders instead of fabricated values

#### Scenario: MQTT is disconnected
- **WHEN** the MQTT client is disconnected
- **THEN** the page labels the broker connection as offline while retaining the latest stored readings

#### Scenario: No plants are discovered
- **WHEN** the catalog is empty
- **THEN** the page explains that it is waiting for readings and renders no fabricated plant cards

### Requirement: Periodic static refresh
Read-only HTML pages SHALL instruct the browser to request a fresh server-rendered snapshot periodically without requiring JavaScript, WebSockets, or server-sent events. A name-edit form response SHALL redirect before normal periodic refresh resumes.

#### Scenario: State changes after a page load
- **WHEN** a new MQTT value arrives after the browser loads a read-only page
- **THEN** a subsequent automatic page refresh displays the updated stored value

### Requirement: Read-only web interface
The web display SHALL expose no operation that changes readings, OLED selection, runtime configuration, MQTT topics, or plant identifiers. Its only state-changing operation SHALL assign or replace the display name of an already discovered plant.

#### Scenario: Unsupported root method is requested
- **WHEN** a client sends a non-read method to the root page
- **THEN** the server rejects the request without changing application state

#### Scenario: Unknown path is requested
- **WHEN** a client requests a path other than a documented overview, plant detail, or plant-name route
- **THEN** the server returns a not-found response

#### Scenario: Unsupported plant method is requested
- **WHEN** a client uses a method not documented for a plant detail or name route
- **THEN** the server returns a method-not-allowed response without changing application state

### Requirement: Consistent concurrent snapshots
HTTP requests SHALL observe race-free copied state and historical query results while MQTT messages, physical button events, and name commands update the application concurrently. Plant name mutations SHALL be serialized by the application coordinator and committed to storage before appearing in snapshots.

#### Scenario: Request overlaps an update
- **WHEN** an HTTP request reads state while an MQTT, button, or name event mutates it
- **THEN** the response contains a self-consistent view and neither operation races or blocks indefinitely

#### Scenario: Name persistence fails
- **WHEN** the database cannot commit a submitted display name
- **THEN** the server returns an error and the previous stored and displayed name remains unchanged

### Requirement: Web display documentation
The project SHALL document HTTP enablement, the address setting, default URL, trusted-LAN mutation assumption, displayed fields, plant naming, history charts, and verification procedure for both Pi and headless Debian deployments.

#### Scenario: Operator verifies the page
- **WHEN** an operator follows the deployment documentation with HTTP enabled
- **THEN** the operator can open the status page, assign a plant name, and view stored history from another LAN device

#### Scenario: Operator disables the page
- **WHEN** an operator follows the documented HTTP-disable procedure
- **THEN** MQTT collection and durable history continue without listening on the HTTP address

## ADDED Requirements

### Requirement: Plant display name assignment
The browser SHALL allow a user to assign or replace the display name of an existing discovered plant using a trimmed name from 1 through 80 Unicode characters. The application SHALL persist the name before updating views and SHALL NOT allow the route to create or rename a plant identifier.

#### Scenario: Valid name is submitted
- **WHEN** a user submits `Money Tree` for the discovered identifier `money-tree`
- **THEN** the name is durably stored, application views use `Money Tree`, and the browser redirects with status 303 to the plant detail page

#### Scenario: Invalid name is submitted
- **WHEN** a submitted name is empty after trimming or exceeds 80 Unicode characters
- **THEN** the server returns status 400 and preserves the previous name

#### Scenario: Unknown plant is named
- **WHEN** a name is submitted for an identifier that has not been discovered
- **THEN** the server returns status 404 and creates neither a plant nor a name

### Requirement: Plant history page
The browser SHALL provide a server-rendered detail page for each discovered plant containing its identifier, assigned display name or identifier fallback, latest reading, name form, and at most the newest 1,000 historical readings rendered chronologically as an inline SVG percentage chart with an accessible tabular fallback. It SHALL use no external assets or JavaScript dependency.

#### Scenario: Known plant has history
- **WHEN** a client sends `GET /plants/money-tree` for a plant with stored readings
- **THEN** the server returns status 200 with its inline 0-to-100-percent chart and timestamped raw and percentage table

#### Scenario: Known plant has more than 1,000 readings
- **WHEN** a detail page is requested for a plant with more than 1,000 stored readings
- **THEN** the chart and table contain only the newest 1,000 readings ordered from oldest to newest

#### Scenario: Unknown plant is requested
- **WHEN** a detail page is requested for an identifier that has not been discovered
- **THEN** the server returns status 404

