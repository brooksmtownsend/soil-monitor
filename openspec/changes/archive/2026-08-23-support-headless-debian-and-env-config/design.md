## Context

The process currently constructs Raspberry Pi GPIO/SPI adapters and the HTTP server unconditionally, while `internal/config` reads only the process environment. The deployed service requires `/etc/soil-monitor/soil-monitor.env`; bootstrap assumes a Debian-family ARM64 Raspberry Pi, installs `raspi-config`, creates hardware groups, enables SPI, and reboots when the SPI device is absent. Release always transfers the ARM64 build.

The desired operating modes range from the existing complete Pi appliance to a headless Debian LXC that runs only MQTT plus the embedded web display. Existing Pi commands and configuration must remain valid, and host-managed environment files must remain preserved.

## Goals / Non-Goals

**Goals:**

- Make Pi HAT and HTTP adapters independently selectable without weakening MQTT state handling or graceful shutdown.
- Support one validated configuration assembled from an optional file, direct environment variables, and command-line arguments.
- Provision and release to systemd-capable Debian hosts on supported Linux architectures, skipping all Pi-only work in no-HAT mode.
- Preserve current behavior when no new switches are supplied.
- Keep configuration parsing and runtime composition testable off the Pi with no new runtime dependency.

**Non-Goals:**

- Supporting non-Debian distributions, non-systemd service managers, containers without SSH/systemd, or remote Docker orchestration.
- Auto-detecting whether a HAT is physically attached; the operator selects the mode explicitly.
- Replacing Mosquitto, adding authentication/TLS, changing the exactly-three-plants contract, or adding persistent reading storage.
- Turning bootstrap into a general-purpose Debian hardening or Proxmox management tool.

## Decisions

### Use one layered runtime configuration model

Add a standard-library command-line parser around the existing typed `Config`. Inputs merge in this order, from lowest to highest precedence: built-in defaults, an explicitly named configuration file, process environment variables, and explicitly supplied command-line flags. Required values are checked only after merging. The main settings include `--config`, `--mqtt-broker`, `--mqtt-client-id`, `--plants`, `--pi-hat`, `--spi-port`, `--button-debounce-ms`, `--http`, and `--http-addr`, with matching `SOIL_*` names.

The optional file uses a deliberately small, non-executable `KEY=VALUE` format compatible with the repository's environment example: blank lines and comments are allowed, values may be unquoted or single/double quoted, and shell expansion or command execution is never performed. An explicitly requested missing or malformed file is an error; without `--config`, no file is required. This keeps direct `SOIL_MQTT_BROKER=... SOIL_PLANTS=... ./soil-monitor` execution simple and avoids a third-party CLI/config dependency. Alternatives considered were YAML/TOML plus a library and sourcing shell files; the former adds unnecessary dependencies and the latter executes untrusted content.

### Model HAT and HTTP as independent feature gates

Introduce `SOIL_PI_HAT_ENABLED`/`--pi-hat` and `SOIL_HTTP_ENABLED`/`--http`, both defaulting to true for backward compatibility. Hardware-only settings are validated only when HAT support is enabled; HTTP address validation occurs only when HTTP is enabled. Boolean flags accept explicit values such as `--pi-hat=false`, which makes scripted bootstrap and binary invocations unambiguous.

The composition root initializes `periph`, SH1106, and buttons only in HAT mode, and creates the web adapter only in HTTP mode. `app.Runner` treats display/buttons as a pair and the web adapter as optional, conditionally starts event sources, redraws only when a display exists, and closes only adapters that were created. MQTT and state remain mandatory. Optional interfaces were chosen over hidden no-op adapters because invalid partial composition remains detectable and resource ownership stays visible.

### Split bootstrap into base Debian and optional Pi-HAT phases

The local bootstrap accepts the same `--pi-hat=<bool>` form and passes the resolved mode to the privileged remote helper. The base phase verifies a Debian-family, systemd-capable host; installs Mosquitto and required administration tools; creates the service account and directories; installs broker, unit, environment, and sudoers assets; and enables services. The Pi phase additionally verifies ARM64, installs `raspi-config`, creates/adds `gpio` and `spi` groups, enables SPI, checks the configured device, and performs the existing bounded reboot flow.

No-HAT mode never requires `raspi-config`, hardware groups, an SPI device, or reboot. Its newly created starter environment explicitly disables HAT support. The systemd unit will not declare unconditional supplementary hardware groups; normal account group membership supplies them on Pi hosts. Existing environment content remains untouched in either mode.

### Select the release artifact from the remote Linux architecture

Provide static Linux ARM64 and AMD64 build artifacts through mise, retaining `build-pi` as the compatible ARM64 entry point. Release performs a read-only `uname -m` probe after SSH preflight, maps `aarch64`/`arm64` to Go `arm64` and `x86_64`/`amd64` to Go `amd64`, and selects the matching prebuilt artifact. Unsupported architectures fail before transfer or remote mutation.

Building both supported artifacts before release is predictable and keeps compilation inside the documented mise workflow. The alternative—cross-building dynamically inside the release shell after probing—would couple build behavior to deployment internals and make dry runs and fixtures less transparent.

### Make the service environment file optional, not implicit configuration

The unit uses systemd's optional `EnvironmentFile=-/etc/soil-monitor/soil-monitor.env`. Release validates readability only when the file exists and never creates or edits it. Bootstrap continues creating a mode-appropriate starter file when absent so managed installs work immediately. A host that removes the file must inject required variables through another systemd-supported mechanism; otherwise application validation causes an explicit startup failure.

## Risks / Trade-offs

- **[Boolean defaults could surprise new headless installs]** → Preserve true defaults for compatibility, but make generic bootstrap write `SOIL_PI_HAT_ENABLED=false` and print the selected mode in dry runs.
- **[Two configuration parsers may interpret files differently]** → Define and test a conservative `KEY=VALUE` subset accepted by both the application loader and systemd, and keep the example inside that subset.
- **[Optional adapters add runner branches]** → Enforce the display/buttons pairing in validation and cover all four HAT/HTTP combinations with unit tests and race testing.
- **[Remote architecture aliases vary]** → Accept only explicit tested aliases and fail before transfer for anything else.
- **[An optional environment file can yield an unconfigured service]** → Keep bootstrap creation as the normal managed path and rely on actionable application validation plus release diagnostics when externally injected variables are absent.

## Migration Plan

1. Add configuration parsing and optional runtime composition with compatibility tests for current environment-only startup.
2. Add supported Linux build artifacts and architecture-aware release selection without changing the default Pi outcome.
3. Refactor bootstrap into base and HAT phases, then update the unit and mode-specific starter configuration.
4. Run Go tests, race tests, release/bootstrap fixtures, vet, shell lint, both Linux builds, and strict OpenSpec validation.
5. Existing Pi hosts require no configuration change. Generic Debian/LXC hosts run bootstrap with `--pi-hat=false`; direct runs can omit `--config` and provide required `SOIL_*` variables.

Rollback consists of restoring the prior binary, unit, and deployment scripts. Existing environment files remain compatible because the new enablement variables default to the prior enabled behavior.

## Open Questions

None. The initial supported generic architectures are Linux ARM64 and AMD64, matching Raspberry Pi 3A+ and typical Proxmox LXC hosts.
