# rpi-release-workflow Specification

## Purpose

Define a repeatable, guarded workflow for validating, building, deploying, activating, diagnosing, and recovering releases of the soil-monitor service on its Raspberry Pi target.
## Requirements
### Requirement: One-command release
The project SHALL provide a `mise run release` command that performs local validation, builds supported static Linux soil-monitor binaries, determines the remote host's supported architecture, deploys the matching artifact, and activates the service in dependency order.

#### Scenario: ARM64 release completes successfully
- **WHEN** the operator runs `mise run release` against a valid ARM64 Pi or Debian target
- **THEN** the command selects the Linux ARM64 artifact and builds, transfers, installs, restarts, verifies, and reports a successful release

#### Scenario: AMD64 release completes successfully
- **WHEN** the operator runs `mise run release` against a valid AMD64 Debian target
- **THEN** the command selects the Linux AMD64 artifact and builds, transfers, installs, restarts, verifies, and reports a successful release

#### Scenario: Local validation fails
- **WHEN** tests, static analysis, shell lint, or a required supported build fails
- **THEN** the release exits non-zero before transferring or changing any remote file

#### Scenario: Remote architecture is unsupported
- **WHEN** the remote architecture does not map to a supported static Linux artifact
- **THEN** release exits before transfer or remote mutation with an architecture diagnostic

### Requirement: Configurable Raspberry Pi target
The release workflow SHALL default to `brooks@192.168.1.95` and SHALL allow the SSH target to be overridden through `SOIL_RELEASE_TARGET`. Remote installation paths and the service name MUST use documented project defaults and MUST reject blank or invalid override values.

#### Scenario: Default target is used
- **WHEN** the operator starts a release without setting `SOIL_RELEASE_TARGET`
- **THEN** every remote operation targets `brooks@192.168.1.95`

#### Scenario: Target is overridden
- **WHEN** the operator sets a valid `SOIL_RELEASE_TARGET`
- **THEN** every transfer and SSH operation uses the overridden target

### Requirement: Release preflight
The release workflow SHALL verify required local commands and artifacts, key-based SSH connectivity, non-interactive remote sudo, required `systemd` support, and the remote architecture before installing release artifacts. If the environment file exists, preflight SHALL verify the configured service account can read it; absence of the optional file SHALL NOT itself fail preflight. Checks for protected host files MUST run as an identity authorized to traverse and read those files rather than treating an unprivileged visibility failure as absence.

#### Scenario: SSH key authentication is unavailable
- **WHEN** batch-mode SSH cannot authenticate to the target
- **THEN** the release exits non-zero with an SSH setup diagnostic before remote installation

#### Scenario: Remote sudo is unavailable
- **WHEN** the target cannot execute required commands through `sudo -n`
- **THEN** the release exits non-zero with a sudo setup diagnostic before changing installed files

#### Scenario: Environment configuration is absent
- **WHEN** `/etc/soil-monitor/soil-monitor.env` does not exist on the target
- **THEN** release continues preflight and reports that runtime configuration must be supplied through another supported mechanism

#### Scenario: Existing environment is not readable by service account
- **WHEN** `/etc/soil-monitor/soil-monitor.env` exists but the configured `soilmonitor` service identity cannot read it
- **THEN** the release exits non-zero with an ownership or mode diagnostic before changing installed files

### Requirement: Staged artifact installation
The release workflow SHALL transfer the matching newly built binary and repository `systemd` unit into a non-privileged staging location before installing them into root-owned destinations with explicit owner and mode settings.

#### Scenario: Artifacts are installed
- **WHEN** preflight, architecture selection, and transfer complete successfully
- **THEN** the compatible binary is installed at `/usr/local/bin/soil-monitor` as an executable and the unit is installed at `/etc/systemd/system/soil-monitor.service` as a root-owned configuration file

#### Scenario: Transfer fails
- **WHEN** either artifact cannot be transferred completely
- **THEN** the release exits non-zero before replacing installed files or restarting the service

### Requirement: Host configuration is preserved
Routine releases MUST NOT transfer, replace, edit, or delete `/etc/soil-monitor/soil-monitor.env`.

#### Scenario: Existing environment differs from repository example
- **WHEN** the Pi contains locally configured broker and plant values
- **THEN** a release leaves those values byte-for-byte unchanged

### Requirement: Service activation and verification
After installing artifacts, the release workflow SHALL run `systemctl daemon-reload`, restart `soil-monitor.service`, and verify within a bounded interval that the service is active. Success MUST be reported only after verification passes.

#### Scenario: New service becomes active
- **WHEN** the installed binary starts and `soil-monitor.service` reaches the active state
- **THEN** the release reports the target and active service as successful

#### Scenario: Service does not become active
- **WHEN** the service fails or does not reach the active state within the verification interval
- **THEN** the release reports failure, prints service status and recent journal diagnostics, and exits non-zero

### Requirement: Failed-release recovery
Before replacing existing installed artifacts, the release workflow SHALL retain recoverable copies. If activation of the new release fails and prior artifacts are available, the workflow SHALL attempt to restore them and restart the previous service version without reporting the failed release as successful.

#### Scenario: Upgrade fails and backups exist
- **WHEN** activation fails after replacing a previously installed binary and unit
- **THEN** the workflow restores both prior artifacts, reloads `systemd`, attempts to restart the prior version, and exits non-zero with the rollback result

#### Scenario: First deployment fails without backups
- **WHEN** activation fails and no prior artifact exists
- **THEN** the workflow reports that automatic rollback was unavailable and exits non-zero with service diagnostics

### Requirement: Effect-free dry run
The release workflow SHALL support `SOIL_RELEASE_DRY_RUN=1` and, in that mode, SHALL print the resolved target, artifacts, installation destinations, and service operations without opening a remote connection or changing remote state.

#### Scenario: Operator previews a release
- **WHEN** the operator runs `SOIL_RELEASE_DRY_RUN=1 mise run release`
- **THEN** local validation and build complete, planned remote actions are displayed, and no SSH or transfer command is executed

### Requirement: Release documentation
The project SHALL document the one-time SSH and sudo prerequisites, supported Debian architectures, normal release command, dry-run command, target overrides, Pi HAT mode selection during bootstrap, optional environment-file behavior, installed paths, failure diagnostics, and manual rollback procedure.

#### Scenario: New operator prepares a target
- **WHEN** an operator follows the release documentation on a newly prepared supported Pi or Debian host
- **THEN** the documented prerequisites are sufficient for a subsequent non-interactive `mise run release`

### Requirement: Bootstrap-assisted first deployment
The release workflow SHALL provide a bootstrap entry point that establishes missing remote packages, accounts, permissions, environment defaults, broker configuration, and boot enablement before invoking the existing guarded release process. SPI configuration and hardware groups SHALL be established only when Pi HAT support is enabled.

#### Scenario: Bootstrap delegates final deployment
- **WHEN** remote base provisioning and any enabled hardware reboot complete successfully
- **THEN** bootstrap invokes the same architecture-aware release implementation used by `mise run release` rather than installing the final binary through a separate path

#### Scenario: Bootstrap fails
- **WHEN** any remote provisioning step fails
- **THEN** bootstrap exits non-zero before invoking the final release and identifies bootstrap as the failed phase

### Requirement: Bootstrap target consistency
The bootstrap workflow SHALL use the same `SOIL_RELEASE_TARGET` default and override as routine releases for every SSH, transfer, reboot, and final release operation.

#### Scenario: Bootstrap target is overridden
- **WHEN** the operator sets a valid `SOIL_RELEASE_TARGET`
- **THEN** bootstrap and final release use that target consistently
