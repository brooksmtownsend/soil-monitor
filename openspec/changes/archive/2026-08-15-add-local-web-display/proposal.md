## Why

The OLED provides an at-a-glance view only when someone is physically near the Raspberry Pi. A minimal web page served by the same Go process will make the current plant readings and MQTT connection state visible to other devices on the local network without introducing a separate application or frontend build.

## What Changes

- Add a small HTTP server to the existing soil-monitor binary.
- Serve a server-rendered HTML page showing all three configured plants, their latest moisture values, and the current MQTT connection state.
- Refresh the page periodically using standard HTML so current in-memory values appear without JavaScript, WebSockets, or a frontend toolchain.
- Add a configurable listen address with a documented local-network default.
- Start and stop the HTTP server with the existing application lifecycle and report listener failures clearly.
- Keep the page read-only; it will not change plant selection, publish MQTT messages, or control the OLED.

## Capabilities

### New Capabilities

- `local-web-display`: Exposes the monitor's current in-memory plant readings and MQTT status as a minimal HTML page from the single Go binary.

### Modified Capabilities

None.

## Impact

- Extends application configuration with an HTTP listen address.
- Adds an HTTP adapter and a read-only snapshot of shared plant state.
- Updates the process lifecycle so HTTP serving, MQTT handling, buttons, and OLED rendering shut down together.
- Updates environment examples, service documentation, and local-network usage instructions.
- Adds no external runtime service, JavaScript dependency, database, authentication system, or frontend build step.
