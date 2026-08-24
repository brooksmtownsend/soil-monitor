## 1. Layered Runtime Configuration

- [x] 1.1 Extend the typed configuration with Pi HAT and HTTP enablement and conditional validation while preserving current defaults.
- [x] 1.2 Implement the non-executable `KEY=VALUE` file loader and merge defaults, explicit file, process environment, and explicit command-line arguments in the specified precedence order.
- [x] 1.3 Add the complete command-line surface and help output, including environment-variable mappings and the environment-only direct-run example.
- [x] 1.4 Add configuration tests for precedence, quoted file values, missing and unsafe files, boolean arguments, conditional hardware/HTTP validation, help, and backward-compatible environment startup.

## 2. Optional Application Adapters

- [x] 2.1 Refactor `app.Runner` to require MQTT and state, enforce the display/buttons pair, and conditionally run, redraw, and close optional HAT and web adapters.
- [x] 2.2 Update the composition root to initialize periph, SH1106, and buttons only in Pi HAT mode and to construct the web server only in HTTP mode.
- [x] 2.3 Expand runner and composition tests across all HAT/HTTP combinations, adapter failures, MQTT updates without a display, and graceful shutdown.

## 3. Generic Linux Build and Release

- [x] 3.1 Add discoverable static Linux ARM64 and AMD64 mise build tasks, retain `build-pi` compatibility, and make release/bootstrap validate both deployment artifacts.
- [x] 3.2 Add read-only remote architecture detection and alias mapping to release, select the matching artifact, and reject unsupported architectures before transfer or mutation.
- [x] 3.3 Make release preflight accept an absent environment file, validate readability when it exists, and update the systemd unit to use an optional environment file without unconditional hardware groups.
- [x] 3.4 Extend release fixtures for ARM64 and AMD64 selection, unsupported architectures, dry-run reporting, absent/readable/unreadable environment files, and unchanged rollback behavior.

## 4. Debian and Raspberry Pi Bootstrap Modes

- [x] 4.1 Add validated `--pi-hat=<bool>` handling to local bootstrap, include the selected mode and architecture behavior in dry-run output, and pass the mode to the privileged helper and final release.
- [x] 4.2 Split the privileged helper into common Debian provisioning and a conditional Pi HAT phase, including systemd/architecture checks and omission of Pi packages, groups, SPI checks, and reboot in no-HAT mode.
- [x] 4.3 Create mode-appropriate initial environment content while preserving every existing environment file byte-for-byte and maintaining ownership/readability checks.
- [x] 4.4 Extend bootstrap fixtures for Pi compatibility, generic AMD64 Debian/LXC behavior, no-HAT omission guarantees, mode-specific initial configuration, idempotency, invalid modes, and unsupported hosts.

## 5. Operator Documentation

- [x] 5.1 Update environment examples and runtime documentation with all inputs, precedence, configuration-file grammar, feature defaults, and direct `./soil-monitor` environment-only commands.
- [x] 5.2 Document Pi and generic Debian/LXC bootstrap commands, SSH target overrides, supported architectures, optional environment-file behavior, dry runs, verification, and rollback expectations.

## 6. Verification

- [x] 6.1 Run formatting, focused Go tests, `go test -race ./...`, and the complete `mise run test` suite.
- [x] 6.2 Run `mise run vet`, `mise run lint-shell`, and both supported static Linux build tasks.
- [x] 6.3 Run strict OpenSpec validation for `support-headless-debian-and-env-config` and resolve every reported issue.
