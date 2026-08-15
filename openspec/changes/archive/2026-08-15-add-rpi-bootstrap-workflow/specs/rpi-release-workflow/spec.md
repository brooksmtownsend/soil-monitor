## ADDED Requirements

### Requirement: Bootstrap-assisted first deployment
The release workflow SHALL provide a bootstrap entry point that establishes missing remote packages, accounts, permissions, SPI configuration, environment, broker configuration, and boot enablement before invoking the existing guarded release process.

#### Scenario: Bootstrap delegates final deployment
- **WHEN** remote bootstrap and any required reboot complete successfully
- **THEN** bootstrap invokes the same validated release implementation used by `mise run release` rather than installing the final binary through a separate path

#### Scenario: Bootstrap fails
- **WHEN** any remote provisioning step fails
- **THEN** bootstrap exits non-zero before invoking the final release and identifies bootstrap as the failed phase

### Requirement: Bootstrap target consistency
The bootstrap workflow SHALL use the same `SOIL_RELEASE_TARGET` default and override as routine releases for every SSH, transfer, reboot, and final release operation.

#### Scenario: Bootstrap target is overridden
- **WHEN** the operator sets a valid `SOIL_RELEASE_TARGET`
- **THEN** bootstrap and final release use that target consistently

## MODIFIED Requirements

### Requirement: Release preflight
The release workflow SHALL verify required local commands and artifacts, key-based SSH connectivity, non-interactive remote sudo, the remote service account's read access to the environment file, and required `systemd` support before installing release artifacts. Checks for protected host files MUST run as an identity authorized to traverse and read those files rather than treating an unprivileged visibility failure as absence.

#### Scenario: SSH key authentication is unavailable
- **WHEN** batch-mode SSH cannot authenticate to the target
- **THEN** the release exits non-zero with an SSH setup diagnostic before remote installation

#### Scenario: Remote sudo is unavailable
- **WHEN** the target cannot execute required commands through `sudo -n`
- **THEN** the release exits non-zero with a sudo setup diagnostic before changing installed files

#### Scenario: Environment configuration is missing
- **WHEN** `/etc/soil-monitor/soil-monitor.env` does not exist on the target
- **THEN** the release exits non-zero with a first-time setup instruction and does not create a guessed environment file

#### Scenario: Environment is not readable by service account
- **WHEN** `/etc/soil-monitor/soil-monitor.env` exists but the configured `soilmonitor` service identity cannot read it
- **THEN** the release exits non-zero with an ownership or mode diagnostic before changing installed files
