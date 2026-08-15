## ADDED Requirements

### Requirement: One-command release
The project SHALL provide a `mise run release` command that performs local validation, builds the Linux ARM64 soil-monitor binary, deploys the release artifacts, and activates the service in dependency order.

#### Scenario: Release completes successfully
- **WHEN** the operator runs `mise run release` with valid local and remote prerequisites
- **THEN** the command builds, transfers, installs, restarts, verifies, and reports a successful release without requiring additional deployment commands

#### Scenario: Local validation fails
- **WHEN** tests, static analysis, or the ARM64 build fails
- **THEN** the release exits non-zero before transferring or changing any remote file

### Requirement: Configurable Raspberry Pi target
The release workflow SHALL default to `brooks@192.168.1.95` and SHALL allow the SSH target to be overridden through `SOIL_RELEASE_TARGET`. Remote installation paths and the service name MUST use documented project defaults and MUST reject blank or invalid override values.

#### Scenario: Default target is used
- **WHEN** the operator starts a release without setting `SOIL_RELEASE_TARGET`
- **THEN** every remote operation targets `brooks@192.168.1.95`

#### Scenario: Target is overridden
- **WHEN** the operator sets a valid `SOIL_RELEASE_TARGET`
- **THEN** every transfer and SSH operation uses the overridden target

### Requirement: Release preflight
The release workflow SHALL verify required local commands and artifacts, key-based SSH connectivity, non-interactive remote sudo, the remote environment file, and required `systemd` support before installing release artifacts.

#### Scenario: SSH key authentication is unavailable
- **WHEN** batch-mode SSH cannot authenticate to the target
- **THEN** the release exits non-zero with an SSH setup diagnostic before remote installation

#### Scenario: Remote sudo is unavailable
- **WHEN** the target cannot execute required commands through `sudo -n`
- **THEN** the release exits non-zero with a sudo setup diagnostic before changing installed files

#### Scenario: Environment configuration is missing
- **WHEN** `/etc/soil-monitor/soil-monitor.env` does not exist on the target
- **THEN** the release exits non-zero with a first-time setup instruction and does not create a guessed environment file

### Requirement: Staged artifact installation
The release workflow SHALL transfer the newly built binary and repository `systemd` unit into a non-privileged staging location before installing them into root-owned destinations with explicit owner and mode settings.

#### Scenario: Artifacts are installed
- **WHEN** preflight and transfer complete successfully
- **THEN** the binary is installed at `/usr/local/bin/soil-monitor` as an executable and the unit is installed at `/etc/systemd/system/soil-monitor.service` as a root-owned configuration file

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
The project SHALL document the one-time SSH and sudo prerequisites, normal release command, dry-run command, target overrides, installed paths, failure diagnostics, and manual rollback procedure.

#### Scenario: New operator prepares the target
- **WHEN** an operator follows the release documentation on a newly prepared Pi
- **THEN** the documented prerequisites are sufficient for a subsequent non-interactive `mise run release`
