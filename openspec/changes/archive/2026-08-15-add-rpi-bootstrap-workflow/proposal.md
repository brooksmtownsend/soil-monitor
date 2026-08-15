## Why

Replacing or reimaging the Raspberry Pi currently requires reconstructing undocumented host setup by hand before the normal release can run. A repeatable bootstrap workflow will turn a freshly imaged, SSH-accessible Pi into the complete OLED display and local MQTT broker appliance without rediscovering packages, users, permissions, SPI configuration, or service files.

## What Changes

- Add a `mise run bootstrap` workflow that validates/builds locally, stages host-setup assets over SSH, provisions the Pi, handles a required SPI reboot, waits for SSH to return, and performs the existing application release.
- Add an idempotent privileged Pi bootstrap script that installs and configures Mosquitto, enables SPI, creates the application account and directories, installs the initial environment and release sudo policy, and enables required services.
- Configure the application to use the Pi-local Mosquitto broker by default while allowing MQTT publishers on the trusted LAN.
- Preserve host-managed environment configuration on repeated bootstrap runs.
- Add automated shell tests and operator documentation for fresh-image prerequisites, bootstrap, security boundaries, and troubleshooting.

## Capabilities

### New Capabilities
- `rpi-host-bootstrap`: Provisioning behavior that prepares a fresh SSH-accessible Raspberry Pi as the complete soil-monitor appliance.

### Modified Capabilities
- `rpi-release-workflow`: The release workflow gains a bootstrap entry point that prepares missing remote prerequisites before invoking the existing guarded release.

## Impact

The change affects `mise` tasks, shell deployment scripts, Debian packages and systemd services on the Pi, Mosquitto configuration, the example application environment, the release sudo policy, tests, and deployment documentation. It does not change MQTT topics, plant state, OLED navigation, or the Go application protocol.
