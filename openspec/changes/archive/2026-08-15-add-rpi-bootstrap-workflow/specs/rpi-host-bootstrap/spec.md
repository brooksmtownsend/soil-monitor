## ADDED Requirements

### Requirement: Fresh-host bootstrap entry point
The project SHALL provide a documented `mise run bootstrap` command that prepares an SSH-accessible, sudo-capable 64-bit Debian/Raspberry Pi OS host and then runs the normal soil-monitor release workflow.

#### Scenario: Fresh host bootstraps successfully
- **WHEN** the operator runs bootstrap with valid SSH, sudo, package repository, and hardware prerequisites
- **THEN** the command provisions the host, releases the application, and reports active soil-monitor and Mosquitto services

#### Scenario: Unreachable host
- **WHEN** the configured SSH target cannot be reached or authenticated
- **THEN** bootstrap exits non-zero before attempting remote host changes and reports the SSH prerequisite

### Requirement: Idempotent privileged bootstrap
The project SHALL provide a standalone Pi bootstrap script that requires root privileges and can be safely rerun after partial or complete provisioning.

#### Scenario: Bootstrap is rerun
- **WHEN** packages, accounts, groups, directories, or managed configuration already exist
- **THEN** bootstrap converges them to the required state without failing solely because they exist

#### Scenario: Bootstrap lacks root privileges
- **WHEN** the remote bootstrap script is executed without root privileges
- **THEN** it exits non-zero before changing the host and explains that sudo or root is required

### Requirement: Local MQTT broker provisioning
Bootstrap SHALL install Mosquitto and its client utilities, configure a listener on TCP port 1883 for trusted-LAN publishers, allow the prototype's anonymous MQTT access, and enable the broker at boot.

#### Scenario: Broker is newly installed
- **WHEN** a fresh host is bootstrapped
- **THEN** Mosquitto is installed, configured, enabled, restarted, and listening according to the repository-managed configuration

#### Scenario: Broker configuration is rerun
- **WHEN** bootstrap runs on an already provisioned host
- **THEN** it reapplies the managed broker configuration and verifies the service without creating duplicate directives

### Requirement: Hardware and service-account provisioning
Bootstrap SHALL enable SPI, ensure the `soilmonitor` system account belongs to the `gpio` and `spi` groups, create required installation directories with explicit ownership and modes, and install the application unit for boot enablement.

#### Scenario: SPI requires reboot
- **WHEN** SPI is enabled in boot configuration but `/dev/spidev0.0` is not present in the running kernel
- **THEN** bootstrap performs one controlled reboot, waits for SSH to return within a bounded interval, and verifies the SPI device before release

#### Scenario: SPI is already active
- **WHEN** `/dev/spidev0.0` exists after bootstrap
- **THEN** bootstrap proceeds without rebooting

### Requirement: Initial environment creation and preservation
Bootstrap SHALL create `/etc/soil-monitor/soil-monitor.env` from the repository example only when it is missing, with root ownership, `soilmonitor` group access, and non-world-readable permissions. The initial broker URL SHALL use the Pi-local Mosquitto listener.

#### Scenario: Environment is missing
- **WHEN** bootstrap runs and `/etc/soil-monitor/soil-monitor.env` does not exist
- **THEN** it installs the example with `SOIL_MQTT_BROKER=tcp://127.0.0.1:1883` and the required ownership and mode

#### Scenario: Environment already exists
- **WHEN** bootstrap runs and the environment file contains host-specific configuration
- **THEN** its content remains unchanged and its ownership and mode are normalized for read access by `soilmonitor`

#### Scenario: Service account cannot read environment
- **WHEN** the provisioned `soilmonitor` identity cannot read `/etc/soil-monitor/soil-monitor.env`
- **THEN** bootstrap exits non-zero before reporting the host ready for release

### Requirement: Bootstrap diagnostics and security boundaries
Bootstrap SHALL validate inputs, use bounded waits, preserve SSH host verification, and report the phase that failed. It MUST NOT configure Wi-Fi, weaken SSH authentication, or expose MQTT beyond the explicitly documented trusted-LAN listener.

#### Scenario: Pi does not return after reboot
- **WHEN** SSH does not become available within the configured bootstrap timeout
- **THEN** bootstrap exits non-zero with a reboot diagnostic and does not claim that release succeeded

#### Scenario: SSH host key changed after reimage
- **WHEN** SSH rejects the target because its host key differs from the saved key
- **THEN** bootstrap leaves host-key verification enabled and directs the operator to verify and resolve the changed key manually
