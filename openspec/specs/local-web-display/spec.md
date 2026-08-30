# local-web-display Specification

## Purpose

Define the embedded local HTTP display, including dynamic plant status, durable naming, historical charts, concurrency, configuration, lifecycle, and operator documentation.

## Requirements

### Requirement: Single-binary HTTP display
The soil-monitor process SHALL optionally serve its web display from the same Go binary that handles MQTT and any enabled OLED and button adapters, without requiring external web assets or an additional runtime service. HTTP enablement SHALL be independent of Pi HAT enablement.

#### Scenario: All displays are enabled
- **WHEN** the process starts with valid Pi HAT, MQTT, and HTTP configuration
- **THEN** the OLED workflow and HTTP listener run within the same process

#### Scenario: Headless web mode starts
- **WHEN** the process starts with Pi HAT support disabled and HTTP enabled
- **THEN** MQTT and the HTTP listener run without accessing Pi hardware

#### Scenario: HTTP is disabled
- **WHEN** the process starts with HTTP disabled
- **THEN** it does not create a network listener for the web display

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

### Requirement: Live WebSocket updates
Read-only HTML pages SHALL connect to the server via a WebSocket endpoint and receive pushed state snapshots whenever readings or connection state change, without a full page reload. Inline JavaScript SHALL be permitted solely to manage this WebSocket connection and apply DOM updates; no external JavaScript libraries or assets are permitted. When the WebSocket is unavailable or closes unexpectedly, the browser SHALL attempt reconnection automatically so the page self-heals after a server restart. A name-edit form response SHALL still redirect to the plant detail page after saving.

#### Scenario: New reading arrives while overview page is open
- **WHEN** a valid MQTT reading is received while one or more browsers have the overview page open
- **THEN** each connected browser updates the affected plant card in place without a full page reload

#### Scenario: MQTT connection state changes while page is open
- **WHEN** the MQTT broker connection changes state while a browser has the overview page open
- **THEN** the MQTT status indicator updates in place without a full page reload

#### Scenario: Browser connects to the WebSocket endpoint
- **WHEN** a browser opens the WebSocket endpoint at `/ws`
- **THEN** the server immediately pushes the current state snapshot to that client

#### Scenario: WebSocket connection is closed or errors
- **WHEN** the WebSocket connection drops or the server restarts
- **THEN** the browser attempts to reconnect periodically until the connection is re-established

#### Scenario: No JavaScript is available
- **WHEN** a browser with JavaScript disabled loads a read-only page
- **THEN** the page still renders the current state as returned by the server-rendered HTML response; live updates are unavailable but the page is not broken

### Requirement: Configurable HTTP listener
The application SHALL independently configure HTTP enablement and `SOIL_HTTP_ADDR`. HTTP SHALL default to enabled, the address SHALL default to `:80`, and a malformed address SHALL be rejected only when HTTP is enabled.

#### Scenario: Default address is used
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` is unset or blank
- **THEN** the HTTP server listens on port 80 on the configured host interfaces

#### Scenario: Address is overridden
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` contains a valid host-and-port listen address
- **THEN** the HTTP server uses the configured address

#### Scenario: Enabled address is invalid
- **WHEN** HTTP is enabled and `SOIL_HTTP_ADDR` does not contain a valid listen port
- **THEN** startup exits non-zero with a configuration diagnostic before starting the application

#### Scenario: Disabled address is absent
- **WHEN** HTTP is disabled and no listen address is supplied
- **THEN** HTTP address validation does not prevent startup

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

### Requirement: HTTP lifecycle and diagnostics
When enabled, the HTTP server SHALL start and stop with the application context, shut down gracefully within a bounded interval, and surface unexpected listener or serving failures as application errors. When disabled, application lifecycle SHALL NOT require a web adapter.

#### Scenario: Enabled service is stopped
- **WHEN** systemd sends the application its termination signal while HTTP is enabled
- **THEN** the HTTP listener stops accepting requests and active responses receive a bounded graceful-shutdown opportunity

#### Scenario: Listener cannot bind
- **WHEN** HTTP is enabled and the configured address is unavailable
- **THEN** the application exits non-zero with an HTTP listener diagnostic

#### Scenario: HTTP is disabled during shutdown
- **WHEN** the application receives a termination signal while HTTP is disabled
- **THEN** it shuts down normally without waiting for or closing a web listener

### Requirement: Web display documentation
The project SHALL document HTTP enablement, the address setting, default URL, trusted-LAN mutation assumption, displayed fields, plant naming, history charts, and verification procedure for both Pi and headless Debian deployments.

#### Scenario: Operator verifies the page
- **WHEN** an operator follows the deployment documentation with HTTP enabled
- **THEN** the operator can open the status page, assign a plant name, and view stored history from another LAN device

#### Scenario: Operator disables the page
- **WHEN** an operator follows the documented HTTP-disable procedure
- **THEN** MQTT collection and durable history continue without listening on the HTTP address

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
The browser SHALL provide a server-rendered detail page for each discovered plant containing its identifier, assigned display name or identifier fallback, latest reading, name form, and controls for the rolling history ranges `1d`, `1w`, `1mo`, `3mo`, `6mo`, and `1y`. The selected range SHALL end at the current request time and begin 24 hours, 7 days, 1 calendar month, 3 calendar months, 6 calendar months, or 1 calendar year earlier in UTC, respectively. Calendar subtraction SHALL preserve the UTC time of day and clamp the day to the last valid day of the target month when necessary. The page SHALL render at most the newest 1,000 readings within the selected range, ordered chronologically as an inline SVG percentage chart with an accessible tabular fallback, and SHALL use no external assets or JavaScript dependency.

#### Scenario: Detail page is requested without a range
- **WHEN** a client sends `GET /plants/money-tree` without a `range` query value
- **THEN** the server returns status 200 with `1d` selected and renders only readings from the rolling 24-hour interval ending at the request time

#### Scenario: Known plant has history
- **WHEN** a client sends `GET /plants/money-tree?range=3mo`
- **THEN** the server returns status 200, identifies `3mo` as selected, provides controls for all six supported presets, and renders only readings observed since the instant three calendar months before the request time

#### Scenario: Calendar range crosses a shorter month
- **WHEN** a calendar-month or calendar-year preset targets a month without the request date's day number
- **THEN** the range begins on the target month's final day at the same UTC time of day

#### Scenario: Known plant has more than 1,000 readings
- **WHEN** the selected range contains more than 1,000 stored readings
- **THEN** the chart and table contain only the newest 1,000 readings within that range, ordered from oldest to newest

#### Scenario: Selected range has no readings
- **WHEN** a known plant has no stored readings within the selected range
- **THEN** the page returns status 200, retains the selected range controls, and explicitly reports that no readings exist in that range

#### Scenario: Unsupported range is requested
- **WHEN** a client requests a plant detail page with an empty or unsupported `range` query value
- **THEN** the server returns status 400 without querying or rendering plant history

#### Scenario: Unknown plant is requested
- **WHEN** a detail page is requested for an identifier that has not been discovered
- **THEN** the server returns status 404
