## 1. Release Task Foundation

- [x] 1.1 Extend `.mise.toml` with reusable test, vet, ARM64 build, and `release` tasks so `mise run release` enforces local checks before deployment.
- [x] 1.2 Add the repository release script with strict shell settings, prefixed configuration variables, default target `brooks@192.168.1.95`, absolute-path validation, and actionable local prerequisite checks.
- [x] 1.3 Implement `SOIL_RELEASE_DRY_RUN=1` output that includes all resolved artifacts, destinations, and service actions while skipping SSH and transfer commands.

## 2. Remote Deployment

- [x] 2.1 Implement batch-mode SSH preflight for connectivity, `sudo -n`, the existing environment file, staging-directory access, and required `systemd` commands.
- [x] 2.2 Implement non-privileged staging and complete transfer of the ARM64 binary and `deploy/soil-monitor.service`, with cleanup and no remote installation after a transfer failure.
- [x] 2.3 Implement backup and root-owned installation of the binary and unit while leaving `/etc/soil-monitor/soil-monitor.env` untouched.
- [x] 2.4 Implement remote unit validation, `systemctl daemon-reload`, service restart, bounded active-state verification, and a success summary.
- [x] 2.5 Implement failure diagnostics and best-effort restoration of `.previous` binary and unit backups, including the no-backup first-deployment case.

## 3. Automated Verification

- [x] 3.1 Add a fake-command test harness for SSH and artifact transfer so release behavior can be verified without contacting the Raspberry Pi.
- [x] 3.2 Add tests for default and overridden targets, invalid overrides, missing prerequisites, dry-run isolation, and local-check failure before transfer.
- [x] 3.3 Add tests for successful command ordering, transfer failure isolation, environment-file preservation, service health failure diagnostics, and rollback requests.
- [x] 3.4 Add shell linting through a `mise`-managed tool and verify the normal test, vet, ARM64 build, and dry-release tasks from a clean workspace.

## 4. Operator and Host Setup

- [x] 4.1 Add a narrowly scoped sudoers example or equivalent root-owned deployment helper for the `brooks` release account and validate its syntax without installing it remotely.
- [x] 4.2 Document one-time SSH key, service account, environment file, sudo, and staging prerequisites for `192.168.1.95`.
- [x] 4.3 Document normal and dry-run releases, target overrides, installed paths, expected success output, diagnostics, `.previous` artifacts, and manual rollback.
- [x] 4.4 Run the effect-free dry release locally and record the separate, explicit command an operator can use for the first live deployment.
