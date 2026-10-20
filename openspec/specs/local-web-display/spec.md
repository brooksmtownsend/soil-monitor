# local-web-display Specification

## Purpose

Define the embedded local HTTP display, including dynamic plant status, durable naming, historical charts, concurrency, configuration, lifecycle, and operator documentation.

## Requirements

### Requirement: Plant status page
The root page SHALL render valid HTML containing every registered or MQTT-discovered plant's assigned display name or identifier, identifying photo when present, latest raw reading and moisture percentage, reading time, and the current MQTT connection state. Each plant SHALL link to its detail page, and the page SHALL provide a link to plant registration.

#### Scenario: All readings are known
- **WHEN** accepted MQTT readings exist for all cataloged plants
- **THEN** `GET /` returns status 200 and shows each plant with its latest raw value, percentage, and reading time

#### Scenario: Restored plant has no reading
- **WHEN** a registered or restored plant has no historical reading
- **THEN** the page shows that plant with explicit unknown placeholders instead of fabricated values

#### Scenario: Plant has a photo
- **WHEN** a cataloged plant has a stored photo
- **THEN** the plant card displays that photo with identifying alternative text

#### Scenario: MQTT is disconnected
- **WHEN** the MQTT client is disconnected
- **THEN** the page labels the broker connection as offline while retaining the latest stored readings

#### Scenario: No plants are discovered
- **WHEN** the catalog is empty
- **THEN** the page explains that a plant can be registered or discovered from a reading and renders no fabricated plant cards

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

### Requirement: Eastern Time presentation
The browser SHALL display every human-readable observation timestamp and history-range boundary in the `America/New_York` time zone, using `EST` or `EDT` according to the represented instant. This SHALL apply consistently to initial server-rendered HTML and later WebSocket-driven updates. Machine-readable timestamp values SHALL continue to identify the same absolute instants with RFC 3339 offsets, and persistence and historical filtering SHALL remain based on UTC instants.

#### Scenario: Summer observation is displayed
- **WHEN** a stored observation falls within Eastern daylight-saving time
- **THEN** overview and history views display its local date and time with the `EDT` abbreviation

#### Scenario: Winter observation is displayed
- **WHEN** a stored observation falls within Eastern standard time
- **THEN** overview and history views display its local date and time with the `EST` abbreviation

#### Scenario: Live observation update is received
- **WHEN** a WebSocket snapshot updates an observation timestamp on an open page
- **THEN** the browser displays the same Eastern date, time, and abbreviation that a fresh server-rendered response would display

#### Scenario: History range is rendered
- **WHEN** a plant history page renders the selected range's beginning and ending instants
- **THEN** both human-readable boundaries are displayed in Eastern Time without changing which stored readings qualify for the range

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
The web display SHALL expose no operation that changes moisture readings, runtime configuration, MQTT topics, or existing plant identifiers. Its state-changing operations SHALL be limited to registering a plant, assigning or replacing a display name, assigning or replacing a watering cadence, confirming watering, and uploading or replacing a plant photo.

#### Scenario: Unsupported root method is requested
- **WHEN** a client sends a non-read method to the root page
- **THEN** the server rejects the request without changing application state

#### Scenario: Unknown path is requested
- **WHEN** a client requests a path other than a documented overview, plant detail, registration, plant name, plant schedule, watering confirmation, or plant photo route
- **THEN** the server returns a not-found response

#### Scenario: Unsupported plant method is requested
- **WHEN** a client uses a method not documented for a plant detail, name, schedule, watering, or photo route
- **THEN** the server returns a method-not-allowed response without changing application state

### Requirement: Consistent concurrent snapshots
HTTP requests SHALL observe race-free copied state, plant care fields, and historical query results while MQTT messages, physical button events, and web commands update the application concurrently. Plant registration, display-name, schedule, watering, and photo mutations SHALL be serialized by the application coordinator and committed to storage before appearing in snapshots.

#### Scenario: Request overlaps an update
- **WHEN** an HTTP request reads state while an MQTT, button, or web command mutates it
- **THEN** the response contains a self-consistent view and neither operation races or blocks indefinitely

#### Scenario: Web mutation persistence fails
- **WHEN** the database cannot commit a submitted registration, display name, schedule, watering confirmation, or photo
- **THEN** the server returns an error and the previous stored and displayed state remains unchanged

#### Scenario: Name persistence fails
- **WHEN** the database cannot commit a submitted display name
- **THEN** the server returns an error and the previous stored and displayed name remains unchanged

### Requirement: HTTP lifecycle and diagnostics
When enabled, the HTTP server SHALL start and stop with the application context, shut down gracefully within a bounded interval, and surface unexpected listener or serving failures as application errors. When disabled, application lifecycle SHALL NOT require a web adapter.

#### Scenario: Enabled service is stopped
- **WHEN** the runtime sends the application its termination signal while HTTP is enabled
- **THEN** the HTTP listener stops accepting requests and active responses receive a bounded graceful-shutdown opportunity

#### Scenario: Listener cannot bind
- **WHEN** HTTP is enabled and the configured address is unavailable
- **THEN** the application exits non-zero with an HTTP listener diagnostic

#### Scenario: HTTP is disabled during shutdown
- **WHEN** the application receives a termination signal while HTTP is disabled
- **THEN** it shuts down normally without waiting for or closing a web listener

### Requirement: Web display documentation
The project SHALL document HTTP enablement, the address setting, default URL, trusted-LAN mutation assumption, displayed fields, plant registration and naming, photo compatibility, watering schedules and confirmation, history charts, and verification procedure for the container deployment.

#### Scenario: Operator verifies the page
- **WHEN** an operator follows the deployment documentation with HTTP enabled
- **THEN** the operator can register a plant, upload a supported photo, manage its watering schedule, confirm watering, assign a plant name, and view stored moisture history from another LAN device

#### Scenario: Operator disables the page
- **WHEN** an operator follows the documented HTTP-disable procedure
- **THEN** MQTT collection and durable plant, care, photo, and moisture history data remain available without listening on the HTTP address

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
The browser SHALL provide a server-rendered detail page for each registered or MQTT-discovered plant containing its identifier, assigned display name or identifier fallback, identifying photo when present, latest reading, name form, watering cadence, next watering date or an explicit unscheduled or as-needed state, watering confirmation control, schedule and photo forms, and controls for the rolling history ranges `1d`, `1w`, `1mo`, `3mo`, `6mo`, and `1y`. The selected range SHALL end at the current request time and begin 24 hours, 7 days, 1 calendar month, 3 calendar months, 6 calendar months, or 1 calendar year earlier in UTC, respectively. Calendar subtraction SHALL preserve the UTC time of day and clamp the day to the last valid day of the target month when necessary. The page SHALL render at most the newest 1,000 readings within the selected range as an inline SVG percentage chart ordered chronologically from oldest to newest and an accessible table ordered from newest to oldest. Readings with equal observation timestamps SHALL appear in reverse receipt order in the table. The page SHALL use no external assets or JavaScript dependency.

#### Scenario: Detail page is requested without a range
- **WHEN** a client sends `GET /plants/money-tree` without a `range` query value
- **THEN** the server returns status 200 with `1d` selected and renders plant identity, care state, and only readings from the rolling 24-hour interval ending at the request time

#### Scenario: Known plant has history
- **WHEN** a client sends `GET /plants/money-tree?range=3mo`
- **THEN** the server returns status 200, identifies `3mo` as selected, provides controls for all six supported presets, renders the chart from oldest to newest, and renders the table from newest to oldest

#### Scenario: Known plant has no schedule
- **WHEN** an MQTT-discovered or migrated plant has no watering cadence
- **THEN** the detail page reports that watering is not scheduled and provides the schedule form without fabricating a next date

#### Scenario: Calendar range crosses a shorter month
- **WHEN** a calendar-month or calendar-year preset targets a month without the request date's day number
- **THEN** the range begins on the target month's final day at the same UTC time of day

#### Scenario: Known plant has more than 1,000 readings
- **WHEN** the selected range contains more than 1,000 stored readings
- **THEN** the chart and table contain only the newest 1,000 readings within that range, with the chart ordered oldest to newest and the table ordered newest to oldest

#### Scenario: Readings share an observation timestamp
- **WHEN** two readings in the selected range have the same observation timestamp
- **THEN** the table shows the reading received later before the reading received earlier

#### Scenario: Selected range has no readings
- **WHEN** a known plant has no stored readings within the selected range
- **THEN** the page returns status 200, retains the plant identity, care state, and selected range controls, and explicitly reports that no readings exist in that range

#### Scenario: Unsupported range is requested
- **WHEN** a client requests a plant detail page with an empty or unsupported `range` query value
- **THEN** the server returns status 400 without querying or rendering plant history

#### Scenario: Unknown plant is requested
- **WHEN** a detail page is requested for an identifier that does not exist
- **THEN** the server returns status 404

### Requirement: Web plant registration
The browser SHALL provide a server-rendered registration form at `GET /plants/new` and accept multipart registration at `POST /plants`. The form SHALL require a display name and supported watering cadence and SHALL allow one optional photo. A successful registration SHALL redirect with status 303 to the new plant's detail page. Invalid fields SHALL return status 400, a derived identifier conflict SHALL return status 409, an oversized photo SHALL return status 413, unsupported or malformed photo content SHALL return status 415, and persistence failure SHALL return status 500 without exposing partial state.

#### Scenario: Registration form is requested
- **WHEN** a client sends `GET /plants/new`
- **THEN** the server returns status 200 with name, watering cadence, and optional photo controls

#### Scenario: Valid registration is submitted
- **WHEN** a client posts valid multipart registration fields for `Money Tree`
- **THEN** the server commits the plant and redirects with status 303 to `/plants/money-tree`

#### Scenario: Derived identifier already exists
- **WHEN** a client posts a registration whose derived identifier belongs to an existing plant
- **THEN** the server returns status 409 and does not change the existing plant

#### Scenario: Photo is too large
- **WHEN** the multipart photo field exceeds 20 MiB of file content
- **THEN** the server returns status 413 without storing a plant or the complete oversized upload

#### Scenario: Photo content is unsupported
- **WHEN** the multipart photo field does not contain a valid JPEG, PNG, or supported HEIC signature
- **THEN** the server returns status 415 without storing a plant or photo

### Requirement: Plant photo delivery and replacement
The browser SHALL display a stored photo on the overview and plant detail page by loading `GET /plants/{id}/photo`. The route SHALL support GET and HEAD, return the stored bytes with their detected media type and nosniff protection, return 404 for an unknown plant or plant without a photo, and reject other methods. The detail page SHALL provide a multipart form that replaces the photo only after a valid upload is committed.

#### Scenario: Stored photo is requested
- **WHEN** a client sends `GET /plants/money-tree/photo` for a plant with a JPEG photo
- **THEN** the server returns status 200 with the original bytes, `Content-Type: image/jpeg`, and `X-Content-Type-Options: nosniff`

#### Scenario: HEAD photo request is sent
- **WHEN** a client sends `HEAD /plants/money-tree/photo` for a plant with a photo
- **THEN** the server returns the same status and representation headers as GET without a response body

#### Scenario: Plant has no photo
- **WHEN** a client requests the photo route for a known plant without a stored photo
- **THEN** the server returns status 404

#### Scenario: Valid replacement is submitted
- **WHEN** a client posts a supported photo no larger than 20 MiB to `/plants/money-tree/photo`
- **THEN** the replacement is committed and the server redirects with status 303 to `/plants/money-tree`

#### Scenario: Invalid replacement is submitted
- **WHEN** a client posts an oversized, unsupported, or malformed replacement
- **THEN** the server returns the corresponding validation status and continues serving the previous photo

### Requirement: Web watering management
The plant detail page SHALL allow a user to assign or replace the watering cadence at `POST /plants/{id}/schedule` and confirm watering at `POST /plants/{id}/water`. A successful mutation SHALL redirect with status 303 to the plant detail page. Invalid cadence SHALL return status 400, unknown plant SHALL return status 404, and persistence failure SHALL return status 500 without changing visible state.

#### Scenario: Schedule is assigned
- **WHEN** a client posts the monthly cadence to `/plants/money-tree/schedule`
- **THEN** the cadence and next date are committed and the server redirects with status 303 to `/plants/money-tree`

#### Scenario: Watering is confirmed
- **WHEN** a client posts to `/plants/money-tree/water`
- **THEN** the watering event and revised next date are committed and the server redirects with status 303 to `/plants/money-tree`

#### Scenario: Unknown plant is watered
- **WHEN** a client posts to the watering route for an identifier that does not exist
- **THEN** the server returns status 404 without storing a watering event

#### Scenario: Watering persistence fails
- **WHEN** the watering event and next date cannot be committed atomically
- **THEN** the server returns status 500 and the detail page continues to represent the previous care state

### Requirement: Responsive plant detail tile layout
The plant detail page SHALL organize current moisture, display-name editing, photo display and replacement, watering controls, and moisture history into visually distinct tiles. The compact tiles SHALL resize into a three-column arrangement when the viewport can accommodate it, a two-column arrangement at intermediate widths, and a single vertically scrolling column on narrow mobile viewports. The moisture-history tile SHALL span the full available row in multi-column arrangements, and no tile, form control, chart, or history table SHALL force horizontal page scrolling at a supported viewport width.

#### Scenario: Plant detail page is viewed on a wide screen
- **WHEN** the available viewport width can comfortably accommodate three compact tiles
- **THEN** the page presents compact plant information and controls in a three-column arrangement and presents moisture history across the full grid width

#### Scenario: Plant detail page is viewed at an intermediate width
- **WHEN** the available viewport width can accommodate two compact tiles but not three
- **THEN** the compact tiles reflow into a two-column arrangement and moisture history continues to occupy the full available row

#### Scenario: Plant detail page is viewed on a narrow mobile screen
- **WHEN** the available viewport width cannot comfortably accommodate two compact tiles
- **THEN** every tile appears in a single vertical reading order that can be reached by normal page scrolling without horizontal page scrolling

#### Scenario: Plant detail content is grouped into tiles
- **WHEN** a known plant's detail page is rendered
- **THEN** current moisture, display-name editing, photo display and replacement, watering controls, and moisture history each appear within an identifiable tile while retaining their existing content and actions

#### Scenario: Historical readings exceed the visible tile area
- **WHEN** the moisture-history table is wider or taller than the immediately visible content area
- **THEN** the history tile keeps the page layout intact and allows the user to reach the chart, table columns, and later rows without clipping content

### Requirement: Hardware-free web display
The soil-monitor process SHALL optionally serve its web display from the same Go binary that handles MQTT, without requiring external web assets or an additional runtime service. The process SHALL NOT access GPIO, SPI, or other Raspberry Pi hardware.

#### Scenario: Web mode starts
- **WHEN** the process starts with valid MQTT and HTTP configuration
- **THEN** MQTT and the HTTP listener run within the same process without accessing hardware devices

#### Scenario: HTTP listener disabled
- **WHEN** the process starts with HTTP disabled
- **THEN** it does not create a network listener for the web display
