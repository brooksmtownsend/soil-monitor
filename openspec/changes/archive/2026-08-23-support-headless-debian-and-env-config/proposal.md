## Why

The soil monitor is currently tied to Raspberry Pi HAT initialization, an ARM64 deployment artifact, and a mandatory systemd environment file even when only MQTT and the web display are needed. Making hardware and configuration sources optional now allows the tested application to run ergonomically as a standalone binary or as a service on a Raspberry Pi, a generic Debian machine, or a Proxmox LXC.

## What Changes

- Add command-line configuration for the soil-monitor binary, including switches that independently enable or disable the Pi HAT and web display.
- Allow runtime configuration to come from an optional configuration file, direct process environment variables, and command-line arguments with a documented precedence order and validation after merging.
- Keep the current Raspberry Pi appliance behavior as the default so existing invocations continue to initialize the HAT and serve the web display.
- Make the application runner tolerate omitted display, button, and web adapters while continuing MQTT ingestion and graceful shutdown.
- Add a bootstrap command-line hardware switch. When Pi HAT support is disabled, provision a generic systemd-capable Debian host without installing Raspberry Pi tooling, creating GPIO/SPI groups, enabling SPI, checking `/dev/spidev0.0`, or rebooting for hardware activation.
- Extend bootstrap and release to select a Linux build compatible with the remote Debian host architecture while retaining the current Linux ARM64 Raspberry Pi path.
- Make the deployed environment file optional at service startup while preserving an existing host-managed file byte-for-byte and retaining bootstrap's ability to create a starter file.
- Update automated fixtures and operator documentation for Raspberry Pi and headless Debian/LXC workflows.

## Capabilities

### New Capabilities

- `runtime-configuration`: Defines command-line, process-environment, and optional configuration-file inputs, their precedence, validation, and direct-execution behavior.

### Modified Capabilities

- `plant-moisture-display`: Makes Pi HAT display and button hardware optional while preserving the three-plant MQTT state workflow.
- `local-web-display`: Makes the embedded HTTP display independently optional at runtime.
- `rpi-host-bootstrap`: Generalizes provisioning to supported Debian hosts and skips every Pi-specific action when HAT support is disabled.
- `rpi-release-workflow`: Supports deployments to compatible Debian architectures and no longer requires an environment file to exist when configuration can be supplied another way.
- `mise-task-workflow`: Replaces the Raspberry-Pi-only build assumption with discoverable supported Linux target builds used by generic deployment.

## Impact

- Runtime composition and validation in `cmd/soil-monitor`, `internal/config`, and `internal/app` will gain optional adapter paths and a command-line/config-file loading layer.
- Bootstrap, release, systemd, sudoers, environment examples, and their shell fixtures will need coordinated updates for hardware mode, optional configuration files, and remote architecture selection.
- Existing Pi installations retain their default behavior; headless Debian/LXC operators gain an explicit no-HAT path and can launch the binary directly with environment variables.
- The implementation should continue to prefer the Go standard library and existing shell/mise tooling; no new runtime dependency is expected.
