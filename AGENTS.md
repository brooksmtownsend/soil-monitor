# Agent Guide

This file is for coding agents working in the wireless-soil-monitor repository. Keep `README.md` short and human-oriented; detailed product behavior belongs in OpenSpec, while implementation detail belongs in code and tests.

## Start by discovering, not assuming

Before changing behavior, inspect these sources in order:

1. `openspec/specs/` for the current product and operational contracts.
2. `openspec/changes/` for any active delta specs, designs, and task lists.
3. `mise tasks` for the supported build, test, and container image commands.
4. The relevant code and tests for implementation details and established patterns.

Do not use the README as the complete technical specification. If documentation and implementation disagree, identify the mismatch and update the appropriate OpenSpec change rather than silently choosing one.

## Repository map

- `cmd/soil-monitor/`: composition root and process lifecycle.
- `internal/config/`: environment parsing and validation.
- `internal/core/`: state, snapshots, domain events, and adapter interfaces.
- `internal/app/`: serialized application coordination across MQTT, history, and HTTP.
- `internal/mqttadapter/`: Paho MQTT connection, reconnect, subscription, and event delivery.
- `internal/webdisplay/`: embedded, read-only standard-library HTTP dashboard.
- `deploy/kubernetes/`: Kustomize base (Deployment, Service, PVC) consumed by the GitOps repository.
- `Dockerfile`: multi-stage build into a distroless, non-root image.
- `.github/workflows/`: CI that tests, builds, and publishes the multi-arch image to GHCR.
- `openspec/specs/`: synchronized main specifications.
- `openspec/changes/`: active and archived change artifacts.

## Important system contracts

- The deliverable is a minimal distroless container image (`linux/amd64` and `linux/arm64`) running one static Go binary as non-root UID 65532. CI publishes it to GHCR; the owner's GitOps repository deploys it to Kubernetes. There is no host bootstrap or SSH release path, and no OLED/HAT hardware support.
- The application discovers any number of kebab-case plant IDs through `plants/+/moisture`; display names are assigned later in the browser.
- Moisture payloads are JSON objects with non-negative integer `raw` and `percentage` from `0` through `100`; malformed and out-of-range messages preserve prior state.
- The MQTT broker is external to this project and set through `SOIL_MQTT_BROKER`. The broker and the dashboard (port 8080 in the image) are trusted-network services without authentication or TLS.
- The dashboard is embedded, server-rendered, and has no external assets or JavaScript dependency. Its only mutation assigns a discovered plant's display name.
- Plant metadata and reading history are stored in local SQLite; retained MQTT publications can add another receipt-time observation after restart.
- The SQLite database lives on a single-writer persistent volume at `/data`; the Deployment must stay at one replica with `Recreate` rollout. Configuration is environment variables only.

Read the corresponding main spec before modifying any of these contracts:

- `plant-moisture-display`
- `local-web-display`
- `runtime-configuration`
- `container-image-build`, `container-image-publish`, `kubernetes-gitops-deployment` (after this change is archived)
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
mise run test            # Go and firmware payload tests
mise run vet             # Go static analysis
mise run fmt             # format Go files
mise run build           # local binary
mise run build-pi        # static Linux ARM64 binary
mise run build-image     # local container image (no push)
```

Before completing a normal code change, run at least the directly affected tests plus `mise run vet`. Changes to the Dockerfile require `mise run build-image` and a run of the image; changes to `deploy/kubernetes/` require `kubectl kustomize deploy/kubernetes` to render. Concurrency changes require `go test -race ./...` where the local platform supports it.

## Deployment boundaries

- Releases are published by `.github/workflows/image.yml` on pushes to `main` and version tags; do not push images or tags manually, and never push to the registry or apply manifests to a cluster merely to test a local change.
- Keep workflow permissions minimal (`contents: read`; `packages: write` only on the publishing job) and use `GITHUB_TOKEN`, not long-lived credentials.
- Keep the pod hardened: non-root, read-only root filesystem, no added capabilities, one replica, `Recreate` strategy.
- The base contains no secrets or environment-specific values; those belong to the GitOps repository's overlays.

## Change discipline

- Preserve existing behavior unless the active delta spec changes it.
- Prefer standard-library Go for small features, especially HTTP, before adding dependencies.
- Keep state mutations centralized through `core.State` and `app.Runner`; HTTP readers use copied snapshots.
- Maintain unit tests and fake adapters so verification runs without a broker or cluster.
- Keep changes minimal in a dirty worktree and do not rewrite unrelated user work.
- Update README only when a human quickstart or user-facing operation changes. Put agent instructions here and normative behavior in OpenSpec.
