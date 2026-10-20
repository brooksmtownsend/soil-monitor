# Proposal

## Why

The appliance is currently delivered by SSH-driven `bootstrap`/`release` scripts that mutate a single Raspberry Pi or Debian host. The owner now runs a Kubernetes cluster managed through GitOps, so releases should be an immutable, versioned container image published by CI and rolled out by the cluster's GitOps controller instead of by privileged remote scripts.

## What Changes

- Add a minimal, non-root, multi-architecture (linux/amd64, linux/arm64) container image built from the existing static Go binary.
- Add a GitHub Actions workflow that tests, builds, and pushes the image to GitHub Container Registry (`ghcr.io`) with immutable version tags.
- Add a Kubernetes base (Kustomize) for the app: single-replica Deployment, Service, and PersistentVolumeClaim for the SQLite database, consumable by the owner's GitOps repository. The image uses distroless, runs non-root, and serves HTTP on an unprivileged port.
- Add a `mise` task for building the image locally.
- **BREAKING**: Remove `mise run bootstrap` / `mise run release`, the `scripts/` bootstrap/release tooling and fixtures, and the `deploy/` systemd, sudoers, env-file, and Mosquitto assets. Pi-host provisioning is no longer supported.
- **BREAKING**: Remove all OLED HAT functionality: `internal/hardware`, `internal/display`, button handling, plant selection state, the `SOIL_PI_HAT_ENABLED`, `SOIL_SPI_PORT`, and `SOIL_BUTTON_DEBOUNCE_MS` settings and matching flags, and the `periph.io` and `golang.org/x/image` dependencies.
- MQTT broker provisioning is out of scope; the app connects to an external or in-cluster broker via `SOIL_MQTT_BROKER`.
- SQLite stays embedded; it is not a separate service.

## Capabilities

### New Capabilities
- `container-image-build`: minimal, non-root, multi-arch image contract and runtime defaults.
- `container-image-publish`: GitHub Actions validation and publication of versioned images to GHCR.
- `kubernetes-gitops-deployment`: Kubernetes base manifests and rollout/persistence behavior for GitOps consumption.

### Modified Capabilities
- `mise-task-workflow`: remove release/bootstrap task requirements; add image build task.
- `plant-reading-history`: persistent data lifecycle moves from host state directory to a persistent volume.
- `local-web-display`: remove HAT/OLED references; shutdown refers to a generic termination signal, not systemd.
- `plant-moisture-display`: remove OLED presentation, button navigation, and connection-indicator requirements; keep MQTT intake, validation, and lifecycle (capability name retained).
- `runtime-configuration`: remove Pi HAT, SPI, and debounce settings and defaults.
- `rpi-host-bootstrap`: removed.
- `rpi-release-workflow`: removed.

## Impact

- Removed: OLED/HAT code and tests, `scripts/*`, `deploy/*` (except possibly retained env documentation), bootstrap/release specs and tasks.
- Added: `Dockerfile`, `.dockerignore`, `.github/workflows/`, `deploy/kubernetes/`, mise task, README/AGENTS.md updates.
- Operational: existing Pi database is not migrated automatically; operators may copy the SQLite file into the PVC (documented in design).
- Security: the broker (1883) and dashboard remain unauthenticated/no-TLS trusted-network services; ingress/auth is the cluster owner's responsibility.
