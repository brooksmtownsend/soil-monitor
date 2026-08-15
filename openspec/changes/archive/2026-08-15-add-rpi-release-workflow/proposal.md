## Why

Releasing the soil-monitor daemon currently requires manually cross-building, copying files, and running several privileged commands on the Raspberry Pi. A repeatable one-command workflow will make iteration on the display and button behavior fast while reducing missed steps and inconsistent deployments.

## What Changes

- Add a `mise run release` task that validates and cross-builds the Linux ARM64 binary before deployment.
- Default releases to the Raspberry Pi at `brooks@192.168.1.95`, while allowing the SSH target and remote paths to be overridden.
- Stage and install the binary and `systemd` unit through SSH using key-based authentication and remote `sudo`.
- Reload `systemd`, restart `soil-monitor.service`, and verify that the service becomes active before reporting success.
- Preserve the Pi's existing environment configuration instead of overwriting it during routine releases.
- Add clear preflight checks, failure diagnostics, dry-run support, and release documentation.
- Keep first-time host provisioning and MQTT broker provisioning outside the release command.

## Capabilities

### New Capabilities

- `rpi-release-workflow`: Build, transfer, install, restart, and verify the soil-monitor service on its Raspberry Pi deployment target through a single `mise` task.

### Modified Capabilities

None.

## Impact

- Extends `.mise.toml` with build, verification, and release tasks.
- Adds a release script and documentation for SSH prerequisites and target overrides.
- Uses the existing `deploy/soil-monitor.service` artifact and ARM64 build output.
- Performs remote writes to `/usr/local/bin/soil-monitor` and `/etc/systemd/system/soil-monitor.service` through `sudo` on `brooks@192.168.1.95`.
- Reloads and restarts `soil-monitor.service`; the existing `/etc/soil-monitor/soil-monitor.env` remains untouched.
