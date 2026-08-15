# Agent Guide

This file is for coding agents working in the wireless-soil-monitor repository. Keep `README.md` short and human-oriented; detailed product behavior belongs in OpenSpec, while implementation detail belongs in code and tests.

## Start by discovering, not assuming

Before changing behavior, inspect these sources in order:

1. `openspec/specs/` for the current product and operational contracts.
2. `openspec/changes/` for any active delta specs, designs, and task lists.
3. `mise tasks` for the supported build, test, bootstrap, and release commands.
4. The relevant code and tests for implementation details and established patterns.

Do not use the README as the complete technical specification. If documentation and implementation disagree, identify the mismatch and update the appropriate OpenSpec change rather than silently choosing one.

## Repository map

- `cmd/soil-monitor/`: composition root and process lifecycle.
- `internal/config/`: environment parsing and validation.
- `internal/core/`: state, snapshots, domain events, and adapter interfaces.
- `internal/app/`: serialized application coordination across display, buttons, MQTT, and HTTP.
- `internal/mqttadapter/`: Paho MQTT connection, reconnect, subscription, and event delivery.
- `internal/hardware/`: Raspberry Pi GPIO, SPI, SH1106 OLED, and HAT buttons.
- `internal/display/`: testable 128x64 OLED layout.
- `internal/webdisplay/`: embedded, read-only standard-library HTTP dashboard.
- `deploy/`: systemd, environment, Mosquitto, and sudoers assets.
- `scripts/`: local release/bootstrap orchestration, privileged Pi setup, and shell fixtures.
- `openspec/specs/`: synchronized main specifications.
- `openspec/changes/`: active and archived change artifacts.

## Important system contracts

- The deliverable is one static Go binary targeting Linux ARM64 on a Raspberry Pi 3A+.
- The supported display is the Waveshare 1.3-inch SH1106 OLED HAT in factory four-wire SPI mode using `/dev/spidev0.0`.
- The application has exactly three configured plants and subscribes to `plants/+/moisture`.
- Moisture payloads are base-10 integers from `0` through `100`; malformed, unknown, and out-of-range messages preserve prior state.
- The Pi-local broker defaults to `tcp://127.0.0.1:1883`. Its port 1883 listener and the port 8080 dashboard are trusted-LAN services without authentication or TLS.
- The dashboard is read-only, embedded in the Go binary, server-rendered, and has no external assets or JavaScript dependency.
- Runtime readings are in memory. Retained MQTT publications restore values after process restart.
- `soilmonitor` is the unprivileged service account. `/etc/soil-monitor/soil-monitor.env` is host-managed, `root:soilmonitor`, mode `0640`.
- Routine releases must preserve the environment file byte-for-byte. Bootstrap may create it when missing and normalize its metadata without replacing existing content.

Read the corresponding main spec before modifying any of these contracts:

- `plant-moisture-display`
- `local-web-display`
- `rpi-host-bootstrap`
- `rpi-release-workflow`
- `mise-task-workflow`

## OpenSpec development workflow

Use small, cohesive changes. A change should introduce one capability or one closely related behavioral adjustment rather than becoming a general project bucket.

1. Run `openspec list --json` and inspect existing active work.
2. Create or continue a named change under `openspec/changes/`.
3. Follow `openspec status --change <name> --json` and the generated artifact instructions.
4. Read every dependency artifact before writing its dependent artifact.
5. Implement tasks in order and mark each checkbox immediately after verification.
6. Run tests proportionate to the change, then strict OpenSpec validation.
7. Sync delta specs intelligently into `openspec/specs/`; preserve main-spec content not mentioned by the delta.
8. Archive only after artifacts, implementation tasks, validation, and spec synchronization are complete.

Keep specifications behavioral and testable. Keep shell mechanics, Go structure, and line-by-line implementation decisions in the design, code, or tests rather than main requirements.

## Supported commands

Install developer tools through `mise`; do not introduce a second tool installer or task runner.

```bash
mise install
mise run                 # list tasks
mise run test            # Go, release, and bootstrap fixtures
mise run vet             # Go static analysis
mise run lint-shell      # ShellCheck for scripts
mise run fmt             # format Go files
mise run build           # local binary
mise run build-pi        # static Linux ARM64 binary
```

Before completing a normal code change, run at least the directly affected tests plus `mise run vet`. Changes to deployment shell scripts also require `mise run lint-shell` and the release/bootstrap fixtures included by `mise run test`. Concurrency changes require `go test -race ./...` where the local platform supports it.

## Deployment boundaries

- `mise run bootstrap` mutates a fresh remote host, installs packages and services, and may reboot it.
- `mise run release` transfers artifacts, installs root-owned files through a narrow sudo policy, restarts the service, and can roll back installed artifacts.
- Never run live bootstrap or release merely to test a local code change. Use `SOIL_BOOTSTRAP_DRY_RUN=1` or `SOIL_RELEASE_DRY_RUN=1` unless the user explicitly authorizes the remote operation.
- Preserve SSH host-key verification. Do not add options that bypass it.
- Do not broaden the sudo policy casually. Keep commands, run-as users, paths, ownership, and modes explicit and update fixtures alongside policy changes.
- Do not edit Wi-Fi configuration as part of bootstrap or release; network reachability and SSH are prerequisites.

## Change discipline

- Preserve existing behavior unless the active delta spec changes it.
- Prefer standard-library Go for small features, especially HTTP, before adding dependencies.
- Keep state mutations centralized through `core.State` and `app.Runner`; HTTP readers use copied snapshots.
- Maintain hardware-independent unit tests and fake adapters so most verification runs off-Pi.
- Keep changes minimal in a dirty worktree and do not rewrite unrelated user work.
- Update README only when a human quickstart or user-facing operation changes. Put agent instructions here and normative behavior in OpenSpec.
