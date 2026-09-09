## Why

Plants currently enter the catalog only after publishing a moisture reading, so plants cannot be organized before a sensor is installed and the application cannot help track routine watering. Adding explicit registration, photos, and durable watering schedules makes the web interface useful as a complete plant-care catalog while preserving sensor-driven moisture history.

## What Changes

- Add a web form that registers a plant from its display name, watering cadence, and optional photo.
- Derive the immutable plant identifier by trimming the submitted name, lowercasing it, and converting it to kebab case; reject registration when the derived identifier is invalid or already exists, including an identifier discovered previously through MQTT.
- Support weekly, biweekly, monthly, and as-needed watering cadences. Compute weekly and biweekly dates at 7- and 14-day intervals, compute monthly dates by calendar month with end-of-month clamping, and leave as-needed plants without a next watering date.
- Persist confirmed watering events and advance the next watering date from the confirmation date for scheduled cadences.
- Accept optional JPEG, PNG, and HEIC uploads up to 20 MiB, validate their media signatures, store their bytes and media type in SQLite, and serve them from the embedded web application.
- Show identifying photos in the web catalog and plant detail view when present. The plant page will prominently present the display name, next watering date or as-needed state, a confirm-watering action, and moisture history.
- Retain MQTT discovery: a valid reading may still create an unknown plant, while a reading for a web-registered identifier updates that existing plant.
- Expand the trusted-LAN web mutation contract to include plant registration and watering confirmation in addition to display-name updates.

## Capabilities

### New Capabilities

- `plant-care-management`: Defines manual plant registration, identifier derivation and uniqueness, stored photos, watering cadences, watering confirmation, and next-date calculation.

### Modified Capabilities

- `local-web-display`: Adds registration, photo delivery, watering controls, and the revised plant detail and overview presentation to the embedded web interface.
- `plant-moisture-display`: Expands the catalog from MQTT-discovered plants to both registered and discovered plants while preserving MQTT updates and OLED behavior.

## Impact

The change affects the SQLite schema and migrations, core plant snapshots and catalog state, serialized application commands, embedded HTTP routes/forms/templates, image responses, startup restoration, WebSocket snapshots, and related unit and integration tests. Uploaded media remains inside the existing database and the application remains a single static Go binary with no external asset service. HEIC bytes are served with their native media type; whether a HEIC image renders is determined by the requesting browser, so JPEG remains the most portable upload format.
