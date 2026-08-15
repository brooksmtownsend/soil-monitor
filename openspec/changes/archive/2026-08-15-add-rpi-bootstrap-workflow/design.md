## Context

The normal release workflow deliberately assumes that the remote Pi already has its service account, environment file, SPI interface, sudo policy, and systemd support. A reimage removes all of those prerequisites, and the Pi is also intended to host Mosquitto rather than depend on another broker. Bootstrap must therefore span local validation, privileged host provisioning, an optional reboot, and the existing unprivileged release flow.

The bootstrap begins only after Raspberry Pi OS/Debian Lite has joined Wi-Fi and the operator can authenticate over SSH as a sudo-capable user. Image creation, Wi-Fi recovery, SSH key enrollment, router configuration, and secrets management are outside the repository's control.

## Goals / Non-Goals

**Goals:**

- Provide one documented `mise run bootstrap` entry point from an SSH-accessible fresh image to an active, boot-enabled application and Mosquitto broker.
- Make remote provisioning safe to rerun and preserve an existing `/etc/soil-monitor/soil-monitor.env`.
- Enable SPI and handle the one reboot required before `/dev/spidev0.0` exists.
- Reuse the existing release validation, installation, health checking, and rollback behavior.
- Provide actionable diagnostics when SSH, sudo, package installation, reboot, or service activation fails.

**Non-Goals:**

- Provision Wi-Fi, create the initial login, enroll an SSH key, or recover an unreachable Pi.
- Secure MQTT for an untrusted network with authentication or TLS.
- Replace Raspberry Pi Imager, manage router DHCP reservations, or support non-Debian package managers.
- Replace host-specific environment values after the initial file has been created.

## Decisions

### Split local orchestration from privileged host provisioning

`scripts/bootstrap.sh` will run on the development machine and coordinate validation, staging, remote bootstrap, reboot detection/waiting, and the existing release script. `scripts/bootstrap-pi.sh` will run as root on the Pi and contain only host provisioning. This keeps privileged operations auditable and allows the Pi script to be copied and run manually when desired.

The alternative of expanding `release.sh` directly was rejected because routine releases must remain non-privileged apart from their narrow sudo policy and must continue preserving host configuration.

### Treat network and SSH as prerequisites

Bootstrap will require a valid `user@host` target, batch-mode SSH authentication, and an interactive sudo-capable account. It will not edit wireless configuration. This gives the workflow a clear boundary and avoids embedding Wi-Fi passwords or weakening SSH host verification.

### Configure a Pi-local broker

The initial application environment will use `tcp://127.0.0.1:1883`. Mosquitto will receive a dedicated `conf.d` file with a listener on port 1883 and anonymous access for the trusted LAN, matching the current prototype publishing model. Documentation will call out that this is not appropriate for an internet-exposed or untrusted network.

The alternative of loopback-only Mosquitto was rejected because moisture publishers must reach the broker from other LAN devices. Authentication and TLS are deferred.

### Preserve host configuration on rerun

The bootstrap will create `/etc/soil-monitor/soil-monitor.env` only when missing, using explicit owner and mode settings. Existing files will not be overwritten. Repository-managed Mosquitto, systemd, and release-policy files may be updated idempotently because they define deployment behavior rather than site-specific values.

### Reboot only when SPI is not yet available

The bootstrap enables SPI through `raspi-config`. After provisioning, the local orchestrator checks for `/dev/spidev0.0`; if absent, it requests one reboot, waits for SSH to disappear and return within bounded timeouts, and verifies the device before releasing. A manual bootstrap run reports that a reboot is required instead of hiding the state.

### Reuse the normal release after provisioning

The bootstrap flow will invoke `scripts/release.sh` after the host passes provisioning checks. This avoids a second implementation for binary installation and service health verification. The privileged Pi script installs and enables the unit without starting it; the release installs the binary, refreshes the unit, and starts it.

## Risks / Trade-offs

- **Anonymous MQTT permits any LAN client to publish or subscribe** → Bind only to the trusted LAN, document the limitation, and leave authenticated MQTT as a later change.
- **Package mirrors or SSH can be temporarily unavailable** → Fail non-zero with the failed phase identified; rerunning is safe.
- **A reboot can change reachability or expose a broken Wi-Fi profile** → Require working Wi-Fi before bootstrap, use bounded wait loops, and never modify Wi-Fi configuration.
- **Interactive sudo complicates automation** → Require operator presence for initial privileged bootstrap; install the existing narrow non-interactive release policy for subsequent releases.
- **A customized remote environment could conflict with the local broker** → Preserve it and print whether it was created or retained so the operator can make an intentional edit.

## Migration Plan

1. Add and test the local bootstrap orchestrator and privileged Pi script.
2. Change the initial environment example and documentation to the Pi-local broker.
3. Run a dry-run/test fixture locally, then exercise bootstrap on a fresh Pi image.
4. If bootstrap fails, correct the reported prerequisite and rerun; idempotent operations preserve completed setup.
5. Routine deployments continue to use `mise run release` after bootstrap completes.

## Open Questions

None. MQTT authentication and physical network-recovery mode remain explicit future work.
