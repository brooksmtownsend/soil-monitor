## 1. Bootstrap Interfaces

- [x] 1.1 Add a `mise run bootstrap` task that runs validation, the ARM64 build, and the bootstrap orchestrator
- [x] 1.2 Add validated bootstrap target, staging path, and bounded reboot wait configuration to the local orchestrator
- [x] 1.3 Stage all bootstrap assets consistently to the selected SSH target and invoke the privileged bootstrap

## 2. Raspberry Pi Bootstrap

- [x] 2.1 Implement root and platform preflight checks in an idempotent standalone Pi bootstrap script
- [x] 2.2 Install Mosquitto packages and an explicit trusted-LAN listener configuration, then enable and verify the broker
- [x] 2.3 Enable SPI and provision the `soilmonitor` account, hardware groups, directories, unit, and release sudo policy
- [x] 2.4 Create the initial root-owned environment for the local broker while preserving existing host configuration
- [x] 2.5 Enable the application unit without starting it before the release binary is installed

## 3. Reboot and Release Integration

- [x] 3.1 Detect whether SPI requires reboot and reboot the target only when necessary
- [x] 3.2 Wait within bounded intervals for SSH disconnect and reconnect, then verify `/dev/spidev0.0`
- [x] 3.3 Invoke the existing guarded release with the identical target and verify both application and broker services
- [x] 3.4 Preserve SSH host-key verification and provide phase-specific errors for bootstrap, reboot, and release failures

## 4. Verification and Documentation

- [x] 4.1 Add fixture-based shell tests for validation, staging order, idempotent environment handling, broker configuration, reboot paths, and failure isolation
- [x] 4.2 Update the environment example and README for the Pi-local broker, fresh-image prerequisites, one-command bootstrap, manual bootstrap, and security limitations
- [x] 4.3 Run shell formatting/linting, bootstrap and release tests, Go tests, vet, and an ARM64 build

## 5. Protected Configuration Permissions

- [x] 5.1 Make release preflight and install checks verify the environment as the configured service account
- [x] 5.2 Extend the narrow sudo policy for the exact service-account environment readability check
- [x] 5.3 Make bootstrap normalize and verify environment metadata without replacing its content
- [x] 5.4 Add regression fixtures and rerun release, bootstrap, shell lint, and strict OpenSpec validation
