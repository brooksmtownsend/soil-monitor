# rpi-host-bootstrap Specification

## Purpose

Define the repeatable provisioning workflow that turns a fresh SSH-accessible Raspberry Pi image into the complete soil-monitor appliance with local MQTT, SPI hardware access, host configuration, and release readiness.
## Requirements
### Requirement: Fresh-host bootstrap entry point
The project SHALL provide a documented `mise run bootstrap` command that prepares an SSH-accessible, sudo-capable, systemd-based 64-bit Debian-family host and then runs the normal soil-monitor release workflow. The command SHALL accept an explicit Pi HAT boolean argument and SHALL preserve enabled Pi HAT behavior by default.

#### Scenario: Raspberry Pi host bootstraps successfully
- **WHEN** the operator runs bootstrap with valid SSH, sudo, package repository, and hardware prerequisites and leaves Pi HAT support enabled
- **THEN** the command provisions the Pi hardware and base services, releases the application, and reports active soil-monitor and Mosquitto services

#### Scenario: Generic Debian host bootstraps successfully
- **WHEN** the operator runs bootstrap with `--pi-hat=false` against a supported Debian server or LXC
- **THEN** the command provisions base services, releases the compatible application binary, and reports active soil-monitor and Mosquitto services without requiring Pi hardware

#### Scenario: Unreachable host
- **WHEN** the configured SSH target cannot be reached or authenticated
- **THEN** bootstrap exits non-zero before attempting remote host changes and reports the SSH prerequisite

### Requirement: Idempotent privileged bootstrap
The project SHALL provide a standalone Debian bootstrap script that requires root privileges, accepts the resolved Pi HAT mode, and can be safely rerun after partial or complete provisioning.

#### Scenario: Bootstrap is rerun
- **WHEN** packages, accounts, groups applicable to the selected mode, directories, or managed configuration already exist
- **THEN** bootstrap converges them to the required state without failing solely because they exist

#### Scenario: Bootstrap lacks root privileges
- **WHEN** the remote bootstrap script is executed without root privileges
- **THEN** it exits non-zero before changing the host and explains that sudo or root is required

#### Scenario: Host is not supported
- **WHEN** the remote host is not a supported 64-bit Debian-family system with systemd
- **THEN** bootstrap exits before installing or changing managed assets with an operating-system diagnostic

### Requirement: Local MQTT broker provisioning
Bootstrap SHALL install Mosquitto and its client utilities, configure a listener on TCP port 1883 for trusted-LAN publishers, allow the prototype's anonymous MQTT access, and enable the broker at boot.

#### Scenario: Broker is newly installed
- **WHEN** a fresh host is bootstrapped
- **THEN** Mosquitto is installed, configured, enabled, restarted, and listening according to the repository-managed configuration

#### Scenario: Broker configuration is rerun
- **WHEN** bootstrap runs on an already provisioned host
- **THEN** it reapplies the managed broker configuration and verifies the service without creating duplicate directives

### Requirement: Hardware and service-account provisioning
Bootstrap SHALL always create the unprivileged `soilmonitor` account and required installation directories with explicit ownership and modes and install the application unit for boot enablement. When Pi HAT support is enabled, bootstrap SHALL additionally require a supported ARM64 Pi host, enable SPI, and add the service account to the `gpio` and `spi` groups. When Pi HAT support is disabled, it SHALL NOT require or change Raspberry Pi hardware facilities.

#### Scenario: SPI requires reboot
- **WHEN** Pi HAT support is enabled, SPI is enabled in boot configuration, and `/dev/spidev0.0` is not present in the running kernel
- **THEN** bootstrap performs one controlled reboot, waits for SSH to return within a bounded interval, and verifies the SPI device before release

#### Scenario: SPI is already active
- **WHEN** Pi HAT support is enabled and `/dev/spidev0.0` exists after bootstrap
- **THEN** bootstrap proceeds without rebooting

#### Scenario: Generic Debian mode is selected
- **WHEN** Pi HAT support is disabled
- **THEN** bootstrap does not install `raspi-config`, create or assign `gpio` or `spi` groups, enable SPI, check an SPI device, or reboot for hardware activation

### Requirement: Initial environment creation and preservation
Bootstrap SHALL create `/etc/soil-monitor/soil-monitor.env` from a mode-appropriate repository example only when it is missing, with root ownership, `soilmonitor` group access, and non-world-readable permissions. The initial broker URL SHALL use the host-local Mosquitto listener, and a no-HAT bootstrap SHALL explicitly disable Pi HAT support in the created file.

#### Scenario: Pi environment is missing
- **WHEN** bootstrap runs in Pi HAT mode and `/etc/soil-monitor/soil-monitor.env` does not exist
- **THEN** it installs the example with `SOIL_MQTT_BROKER=tcp://127.0.0.1:1883`, Pi HAT enabled, and the required ownership and mode

#### Scenario: Headless environment is missing
- **WHEN** bootstrap runs with Pi HAT support disabled and `/etc/soil-monitor/soil-monitor.env` does not exist
- **THEN** it creates the initial local-broker configuration with Pi HAT support explicitly disabled

#### Scenario: Environment already exists
- **WHEN** bootstrap runs and the environment file contains host-specific configuration
- **THEN** its content remains unchanged and its ownership and mode are normalized for read access by `soilmonitor`

#### Scenario: Service account cannot read environment
- **WHEN** the provisioned `soilmonitor` identity cannot read a bootstrap-managed environment file
- **THEN** bootstrap exits non-zero before reporting the host ready for release

### Requirement: Bootstrap diagnostics and security boundaries
Bootstrap SHALL validate inputs, print the selected hardware mode and remote architecture in its plan, use bounded waits, preserve SSH host verification, and report the phase that failed. It MUST NOT configure Wi-Fi, weaken SSH authentication, or expose MQTT beyond the explicitly documented trusted-LAN listener.

#### Scenario: Pi does not return after hardware reboot
- **WHEN** Pi HAT mode requires a reboot and SSH does not become available within the configured bootstrap timeout
- **THEN** bootstrap exits non-zero with a reboot diagnostic and does not claim that release succeeded

#### Scenario: SSH host key changed after reimage
- **WHEN** SSH rejects the target because its host key differs from the saved key
- **THEN** bootstrap leaves host-key verification enabled and directs the operator to verify and resolve the changed key manually

#### Scenario: Operator previews generic bootstrap
- **WHEN** the operator performs a no-HAT dry run
- **THEN** the plan identifies generic Debian mode and omits any planned SPI activation or hardware reboot
