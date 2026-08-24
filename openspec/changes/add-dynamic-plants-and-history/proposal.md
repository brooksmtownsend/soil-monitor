## Why

The monitor is currently limited to three preconfigured plants and only retains each plant's latest percentage in memory, which makes adding sensors cumbersome and prevents users from understanding moisture trends. Dynamic discovery, durable names, and local reading history will let the appliance grow with a user's plants while remaining self-contained on the trusted LAN.

## What Changes

- **BREAKING** Replace the integer-only MQTT moisture payload with a JSON object containing the sensor's raw reading and calculated percentage.
- **BREAKING** Remove the requirement to configure exactly three plants; discover any number of plant IDs from the `plants/+/moisture` wildcard subscription.
- Persist discovered plants, user-assigned display names, and accepted moisture readings in a local SQLite database.
- Allow a user to assign or change a display name for a discovered plant ID after readings arrive.
- Extend the browser interface with per-plant historical charts and the plant naming workflow while preserving status visibility for every discovered plant.
- Generalize OLED selection and navigation from a fixed three-plant collection to the dynamically discovered plant collection.
- Update the wireless publisher to emit the new JSON payload without changing its retained publication or power-cycle behavior.
- Add runtime configuration for the local database path and remove the required three-plant definition.

## Capabilities

### New Capabilities

- `plant-reading-history`: Durable storage and retrieval of discovered plants, assigned names, and timestamped moisture readings.

### Modified Capabilities

- `plant-moisture-display`: Accept JSON readings, dynamically discover plants, and navigate any number of discovered plants.
- `local-web-display`: Show discovered plants, support name assignment, and render historical charts from stored readings.
- `runtime-configuration`: Replace required fixed plant configuration with local database configuration.
- `wireless-moisture-publisher`: Publish raw and percentage values as a JSON moisture payload.

## Impact

The core state and event model, MQTT adapter, application runner, OLED rendering/navigation, HTTP routes/templates, configuration parsing, process composition, publisher firmware, deployment configuration, and tests will change. The binary gains an embedded SQLite driver and a host-managed database file whose directory, ownership, backup, and release preservation must be covered by deployment behavior. Existing integer publishers and fixed `SOIL_PLANTS` configurations require migration to the JSON contract.
