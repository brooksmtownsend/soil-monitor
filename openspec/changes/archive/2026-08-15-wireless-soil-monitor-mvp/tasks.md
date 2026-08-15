## 1. Project Foundation

- [x] 1.1 Initialize the Go module, application entry point, package layout, and repeatable build/test commands, using `mise` for any missing developer tool installation.
- [x] 1.2 Confirm the Waveshare HAT revision and SPI/control pin mapping, then select and pin compatible MQTT, GPIO/SPI, and SH1106 Go dependencies.
- [x] 1.3 Implement runtime configuration for the broker URL and exactly three uniquely identified plants, including actionable validation errors and configuration unit tests.

## 2. State and MQTT

- [x] 2.1 Implement the single-owner in-memory state model for optional latest values, initial selection, wrapped previous/next navigation, and MQTT connection state.
- [x] 2.2 Implement strict parsing for `plants/<plant-id>/moisture` messages with base-10 values from 0 through 100, preserving state for malformed, out-of-range, or unknown-plant messages.
- [x] 2.3 Add unit tests covering valid updates, invalid messages, selection behavior, wraparound, and preservation of values across connection-state changes.
- [x] 2.4 Implement the MQTT adapter with `plants/+/moisture` subscription, automatic reconnect/resubscribe behavior, event delivery, and clean disconnect.

## 3. Display and Buttons

- [x] 3.1 Define testable display and button interfaces plus the immutable OLED view model used by the application state owner.
- [x] 3.2 Implement and test the 128x64 layout for plant name, percentage or no-reading placeholder, position out of three, deterministic name truncation, and disconnected indication.
- [x] 3.3 Implement the SH1106 SPI display adapter for the confirmed Waveshare pin mapping, including initialization, frame updates, and resource cleanup.
- [x] 3.4 Implement active-low button watching for BCM GPIO 21, 20, and 16 with pull-ups, falling-edge activation, debounce, previous/next mapping, and reserved-button no-op behavior.

## 4. Application Integration

- [x] 4.1 Wire configuration, hardware adapters, MQTT events, application state, and render triggers into one Go binary with serialized state transitions.
- [x] 4.2 Add startup error handling and signal-driven shutdown that stops button watchers, disconnects MQTT, and releases display and GPIO resources.
- [x] 4.3 Add integration tests with fake display, buttons, and MQTT adapters for startup, selected and non-selected updates, navigation, disconnect/reconnect display changes, and shutdown.

## 5. Raspberry Pi Verification and Operation

- [x] 5.1 Perform a Raspberry Pi smoke test for OLED initialization, readable layout, all three buttons, debounce, and navigation wraparound.
- [x] 5.2 Verify end-to-end MQTT updates from another computer for all three configured plant topics, including malformed-message and broker-reconnect cases.
- [x] 5.3 Add Debian Lite setup and bench-test documentation covering SPI access, configuration, build/install, example `mosquitto_pub` commands, troubleshooting, and the trusted-LAN security assumption.
- [x] 5.4 Add a `systemd` service and environment-file example, verify restart and graceful stop behavior, and document installation and rollback.
