## Context

The target is a Raspberry Pi 3A+ running 64-bit Debian Lite with a Waveshare 1.3-inch SH1106 OLED HAT. The HAT uses SPI for the display and exposes three active-low buttons on BCM GPIO 21, 20, and 16. Moisture measurements will initially be published manually to an MQTT broker so the display and interaction model can be proven before wireless sensors and historical storage are introduced.

The repository has no application code or prior specifications. The first implementation therefore needs to establish a small Go project, hardware boundaries that can be tested without a Pi, and a deployment path for one long-running binary.

## Goals / Non-Goals

**Goals:**

- Produce one Go binary that owns MQTT subscription, current plant state, OLED rendering, and button input.
- Support exactly three configured plants in the MVP while keeping plant identity separate from display names.
- Make message parsing and navigation deterministic and testable without physical hardware.
- Remain usable through broker disconnects and malformed messages.
- Provide a simple Debian installation, configuration, and manual publishing workflow.

**Non-Goals:**

- Reading a moisture probe directly on the Raspberry Pi.
- Persisting readings across process restarts or displaying historical trends.
- Alerts, thresholds, calibration, authentication management, or a remote configuration API.
- Using the HAT joystick or assigning behavior to the third button.
- Managing or provisioning the MQTT broker as part of the application.

## Decisions

### Single-process event-driven application

The application will be a single Go process with separate adapters for MQTT, display, and buttons. Adapter events feed one state owner, which serializes plant updates and selection changes before requesting a render. This avoids shared mutable display state and keeps shutdown behavior predictable.

An alternative of separate subscriber and display services was rejected because it would add inter-process coordination without providing value for three in-memory readings.

### Stable topic and payload contract

The application will subscribe to `plants/+/moisture`. The topic segment between `plants/` and `/moisture` is the plant identifier. Payloads are base-10 integers from 0 through 100 inclusive. Unknown plant identifiers, malformed payloads, and out-of-range values are ignored and logged; they do not erase the last valid reading.

A JSON envelope was considered but rejected for the MVP because the only required field is the moisture percentage. The topic already carries plant identity and a plain integer is easy to publish during bench testing.

### Explicit startup configuration

Runtime configuration will provide the MQTT broker URL and exactly three plant entries, each with a unique kebab-case identifier and a display name. Defaults may target a local unauthenticated broker for development, but values will be overrideable without rebuilding the binary. Invalid or duplicate plant configuration will fail at startup with a useful error.

Dynamic configuration over MQTT was rejected because it complicates validation, persistence, and recovery before those behaviors are needed.

### In-memory latest-value model

Each configured plant stores an optional latest moisture value for the lifetime of the process. Before the first valid message, the display uses a clear no-reading placeholder. MQTT retained messages are supported naturally by subscribing normally, but the application will not require publishers to retain messages.

Local storage was rejected for this change because restart persistence and time-series behavior require separate retention rules that have not yet been defined.

### Small view model for the OLED

Rendering will be based on an immutable view model containing the selected plant name, optional moisture percentage, selected position, total count, and MQTT connection state. The 128x64 layout will prioritize readable text: plant name, value or placeholder, position such as `1 / 3`, and a compact connection indicator when disconnected.

The renderer will redraw on startup, accepted MQTT updates for the selected plant, selection changes, and MQTT connection-state changes. Hardware-specific drawing stays behind a display interface so state and formatting can be unit tested with a fake implementation.

### Button mapping and debounce

BCM GPIO 21 selects the previous plant, GPIO 20 selects the next plant, and GPIO 16 is reserved and has no visible effect. Previous and next navigation wrap at the ends. Inputs use pull-ups and falling-edge activation with software debounce so one press produces one navigation action.

Using the joystick was rejected to keep the first interaction model limited to the three labeled keys requested for the project.

### Hardware integration and lifecycle

SPI/GPIO access will use a maintained Go hardware abstraction suitable for Linux GPIO and SPI, with SH1106-specific commands contained in the display adapter. The exact module versions will be pinned in `go.mod`. The application will initialize hardware before subscribing, handle termination signals, stop button watchers, disconnect MQTT cleanly, and blank or close the display as supported by the selected driver.

The binary will run under `systemd` after manual bench testing. Configuration will be supplied through a dedicated environment file, and the service will restart after transient failures.

## Risks / Trade-offs

- **SH1106 driver or pin mapping differs across HAT revisions** → Confirm the HAT revision and exercise display initialization with a minimal hardware smoke test before completing the full renderer.
- **Blocking hardware calls could delay message handling** → Serialize rendering but keep MQTT callbacks limited to parsing and event delivery.
- **Button bounce causes repeated navigation** → Apply a configurable short debounce window and test rapid edge sequences.
- **An unauthenticated local broker is unsuitable outside a trusted LAN** → Keep credentials and TLS out of the MVP contract, document the trusted-network assumption, and allow a broker URL that can later be extended with secure options.
- **In-memory values disappear on restart** → Display the no-reading state until new or retained MQTT values arrive; persistence remains a later change.
- **Long plant names do not fit 128 pixels** → Define deterministic truncation in the renderer and preserve full names in configuration and logs.

## Migration Plan

There is no existing application to migrate. Implementation will proceed through desktop unit tests, a Raspberry Pi display/button smoke test, MQTT bench publishing, and finally installation as a `systemd` service. Rollback consists of stopping and disabling the service and removing the installed binary and service configuration; the external broker and published messages are unaffected.

## Open Questions

- Confirm the exact Waveshare HAT revision and its display control-pin mapping during the hardware smoke-test task.
- Choose and pin the Go SPI/GPIO and SH1106 modules after verifying compatibility with 64-bit Debian on the Raspberry Pi 3A+.
