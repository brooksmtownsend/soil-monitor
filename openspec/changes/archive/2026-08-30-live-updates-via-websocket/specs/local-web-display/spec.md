## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Periodic static refresh
**Reason**: Replaced by WebSocket live updates, which deliver state changes without a full-page reload and eliminate the visible screen flash caused by meta-refresh cycling.
**Migration**: The `<meta http-equiv="refresh">` tag is removed from both the overview and detail HTML templates. Browsers that previously relied on the automatic reload now receive updates over the WebSocket connection at `/ws`.
