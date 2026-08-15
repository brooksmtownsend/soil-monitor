## Context

The project already cross-builds `build/soil-monitor-linux-arm64` and contains `deploy/soil-monitor.service`, but deployment still consists of several manual local and remote commands. The target Raspberry Pi 3A+ has the static address `192.168.1.95`; prior project context identifies the login user as `brooks`. Routine releases need privileged installation and service control, while the broker and application environment remain host-managed state.

The workflow runs from the developer machine, where `mise` already manages Go. The Pi is expected to have SSH, `systemd`, a configured `soilmonitor` service account, `/etc/soil-monitor/soil-monitor.env`, and narrowly scoped non-interactive sudo access for installation and service management.

## Goals / Non-Goals

**Goals:**

- Make `mise run release` the complete routine release command.
- Validate the code, build a static Linux ARM64 binary, transfer the binary and unit, install them into root-owned paths, and restart the service.
- Fail quickly when local tools, artifacts, SSH connectivity, remote privileges, or required host configuration are missing.
- Verify the new service is active and provide useful diagnostics when it is not.
- Preserve the prior deployable files long enough to restore them after a failed restart.
- Make the workflow testable without modifying the Raspberry Pi.

**Non-Goals:**

- Provisioning Debian, users, SSH keys, MQTT, network settings, or the initial environment file.
- Building on the Raspberry Pi.
- Deploying to multiple hosts concurrently.
- Managing secrets or replacing `/etc/soil-monitor/soil-monitor.env` during a routine release.
- General-purpose remote execution or an artifact registry.

## Decisions

### `mise` is the public release interface

`.mise.toml` will define the public `release` task and reusable validation/build tasks. The release task will run tests and static analysis, cross-build with `GOOS=linux`, `GOARCH=arm64`, and `CGO_ENABLED=0`, then invoke a repository-owned deployment script.

Putting every remote command directly into TOML was considered but rejected because error handling, cleanup, dry-run behavior, and rollback are clearer and testable in a dedicated script. The script is an implementation detail; `mise run release` remains the operator interface.

### One explicit, overrideable target

The default SSH target will be `brooks@192.168.1.95`. `SOIL_RELEASE_TARGET` can override it for a replacement host or alternate account. Other prefixed variables can override the service name and remote install paths for testing, but production defaults remain:

- Binary: `/usr/local/bin/soil-monitor`
- Unit: `/etc/systemd/system/soil-monitor.service`
- Service: `soil-monitor.service`
- Environment: `/etc/soil-monitor/soil-monitor.env`

Every variable will be quoted as data, and required remote install paths must be absolute. A generic inventory system was rejected because the project has one known target.

### Key-based SSH and non-interactive sudo are preconditions

Remote calls will use SSH batch mode so missing key authentication fails instead of hanging for a password. Privileged commands will use `sudo -n`; the release documentation will describe the narrowly scoped sudo policy that the operator must install during one-time host preparation.

Interactive SSH and sudo prompts were considered but rejected because they make automation unreliable and make failures difficult to test.

### Stage as the login user, install as root

The workflow will create or clean a target-specific staging directory owned by the SSH user, transfer the binary and unit there, and then run a single remote installation sequence. Root-owned destinations will only be changed with `sudo install`, using explicit modes and owners. The existing environment file will be checked but never transferred or modified.

Direct transfer into `/usr/local/bin` and `/etc/systemd/system` was rejected because it would require root SSH or privileged transfer tooling.

### Backup and rollback around service activation

Before replacement, existing binary and unit files will be copied to predictable `.previous` backups. After installation, the workflow will run `systemctl daemon-reload`, restart the service, and require `systemctl is-active --quiet` to succeed within a bounded retry window.

If activation fails and previous files exist, the remote sequence will restore both backups, reload `systemd`, and restart the previous release. Regardless of rollback outcome, the release command will return non-zero and print service status plus recent journal entries. On a first deployment without backups, failure diagnostics are reported without claiming rollback.

Leaving failed files in place without recovery was rejected because display development is likely to involve frequent releases to a continuously running device.

### Dry-run and command fakes provide local verification

`SOIL_RELEASE_DRY_RUN=1 mise run release` will run local validation/build steps and print the target, artifacts, and remote actions without connecting to or mutating the Pi. The deployment script will resolve standard tools through `PATH`, allowing automated tests to substitute fake `ssh` and transfer commands and verify sequencing, failure propagation, config preservation, and rollback requests.

A dry run that still connects remotely was rejected because the safest preview has no external effect.

## Risks / Trade-offs

- **Passwordless sudo broadens remote privileges** → Document a narrow command/path policy for the `brooks` account and require `sudo -n` preflight checks.
- **A network interruption occurs after file installation** → Keep `.previous` files on the Pi and document the manual rollback command; automatic rollback is attempted when the remote session remains available.
- **The service briefly stops during restart** → Accept a short interruption for this single-device MVP; zero-downtime deployment is unnecessary.
- **A bad unit passes transfer but fails to load** → Validate locally where possible, run remote `systemd-analyze verify`, and restore the previous unit on activation failure.
- **The environment file is missing on first deployment** → Fail preflight with a setup instruction rather than creating or guessing configuration.
- **Shell quoting or target overrides cause unsafe commands** → Use prefixed variables, reject blank or non-absolute paths, quote every expansion, and cover generated remote actions with fake-command tests.

## Migration Plan

1. Add the `mise` tasks, release script, tests, and release documentation without changing the running Pi.
2. Install the documented SSH key and narrow sudo policy once on `brooks@192.168.1.95`.
3. Confirm the existing environment file and service account, then run a dry release.
4. Run the first live `mise run release` and verify the OLED, MQTT updates, service status, and journal.
5. Use `mise run release` for subsequent application and unit updates.

Rollback uses the remote `.previous` binary and unit created during release, followed by `systemctl daemon-reload` and restart. Removing the local release task does not change the installed Pi service.

## Open Questions

- Confirm that `brooks` is the intended SSH account on `192.168.1.95`; `SOIL_RELEASE_TARGET` remains available if it differs.
- Confirm whether the Pi already permits narrowly scoped `sudo -n` service management or needs the documented one-time sudoers setup.
