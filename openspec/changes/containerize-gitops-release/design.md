# Design

## Context

The app is one static Go binary (`cmd/soil-monitor`) configured by `SOIL_*` environment variables (see `runtime-configuration`). It already supports a headless mode (`SOIL_PI_HAT_ENABLED=false`), an HTTP dashboard on `SOIL_HTTP_ADDR` (default `:80`), an MQTT client, and an embedded pure-Go SQLite database (`modernc.org/sqlite`, no CGO) that also stores plant photos as BLOBs. Existing delivery uses `scripts/` + `deploy/` for SSH bootstrap/release. See proposal.md for motivation.

## Goals / Non-Goals

**Goals:**
- One reproducible, minimal image; CI publishes it to GHCR; a Kustomize base lets the GitOps repo roll it out.
- No application code change required beyond what tests reveal.

**Non-Goals:**
- Preserving any OLED/HAT/button capability; it is removed entirely.
- Deploying or securing an MQTT broker, ingress, TLS, or authentication.
- Moving off SQLite or horizontal scaling.
- Automatic migration of an existing Pi database.

## Decisions

- **SQLite stays embedded, not a separate service.** It is a library writing one file; it needs a PVC and a single writer, not its own workload. Alternative: Postgres/other server DB — requires rewriting `internal/history` and adds an operational dependency for a one-replica hobby app. Deployment uses `strategy: Recreate` and `replicas: 1` so two pods never open the file (SQLite over network filesystems with concurrent writers is unsafe). Prefer a block-storage `ReadWriteOnce` (or `ReadWriteOncePod`) volume, not NFS.
- **Image**: multi-stage `Dockerfile`: `golang` builder (`CGO_ENABLED=0`, `-trimpath`) → `gcr.io/distroless/static:nonroot` (UID/GID 65532, includes CA certs, no shell). Alternative `scratch`: smaller but needs manual CA certs/user/data dir; distroless static is near-equivalent size with less to maintain. Data directory `/data` created and owned by 65532 in the builder stage and copied with ownership so unmounted runs still work. Cross-compilation uses `$BUILDPLATFORM` builder with `GOOS/GOARCH` from `TARGETOS/TARGETARCH` to avoid QEMU-emulated compiles.
- **Image env defaults**: `SOIL_PI_HAT_ENABLED=false`, `SOIL_HTTP_ADDR=:8080`, `SOIL_DATABASE_PATH=/data/soil-monitor.db`. Port 8080 avoids needing `CAP_NET_BIND_SERVICE`; the Service maps port 80 → 8080. `SOIL_MQTT_BROKER` has no image default (config already requires it).
- **Remove OLED/HAT support entirely** (user decision: unused and never to be used): delete `internal/hardware` and `internal/display`; drop the `core.Display`/`ButtonSource`/`Button` ports, `View`, `Previous`/`Next`, the selection index and `Snapshot.SelectedPlantID`; remove the Display/Buttons wiring from `app.Runner` and `main.go`; remove the web template's `selected` class; remove the HAT/SPI/debounce config fields, flags, and help text; run `go mod tidy` to drop `periph.io` and `golang.org/x/image`. The `plant-moisture-display` capability keeps its name since it still owns MQTT intake and validation.
- **CI**: single `.github/workflows/image.yml`: job `test` (setup-go from `go.mod`, `go vet`, `go test ./...`); job `image` needs `test`, uses `docker/setup-qemu-action`/`setup-buildx-action`, `docker/metadata-action` for tags/labels, `docker/login-action` with `GITHUB_TOKEN` (skipped on PRs), `docker/build-push-action` with `platforms: linux/amd64,linux/arm64`, `push` only on non-PR events, provenance enabled, GHA cache. Tags: `sha-<short>` on default branch, `X.Y.Z` on `vX.Y.Z` tags. Third-party actions pinned by major version at minimum (SHA pinning preferred). `permissions: contents: read` top-level; `packages: write` only on the publish job. Package visibility defaults to private; the cluster needs an image pull secret, or the owner makes the package public (documented, not automated).
- **Kubernetes base** in `deploy/kubernetes/` (`kustomization.yaml`, `deployment.yaml`, `service.yaml`, `pvc.yaml`). Assumption: the GitOps repo is separate (Flux/Argo agnostic) and references this directory as a remote base (`github.com/<owner>/wireless-soil-monitor//deploy/kubernetes?ref=<tag>`) or copies it. Overlays patch image tag, `SOIL_MQTT_BROKER`, storage class, and add Ingress/HTTPRoute. Pod securityContext: `runAsNonRoot`, `runAsUser/Group 65532`, `fsGroup 65532`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, drop ALL, `seccompProfile: RuntimeDefault`. Probes: HTTP GET `/` on the named port (verify path during implementation). Resource requests/limits small (e.g. 50m/64Mi).
- **Removal**: delete `scripts/`, `deploy/soil-monitor*` systemd/sudoers/env/mosquitto assets, `lint-shell`/`release`/`bootstrap` mise tasks, and Pi-host sections of README/AGENTS.md. `build-pi`/arm64/amd64 mise tasks stay (the Dockerfile may use its own `go build`; mise tasks remain for local binaries).
- **Env-file loader** (`runtime-configuration` optional config file) remains untouched; Kubernetes uses plain env vars.

## Risks / Trade-offs

- [Data loss on volume mishap] → single PVC, document backup (`sqlite3 .backup` or volume snapshots) and a one-time copy procedure from the Pi (`/var/lib/soil-monitor/soil-monitor.db`).
- [Two pods write the DB during rollout] → `Recreate` strategy, `replicas: 1`; GitOps tools must not be configured to scale it.
- [Unauthenticated dashboard/mutation exposed by ingress] → base ships ClusterIP only; document that auth/TLS belongs to the ingress layer.
- [ARM64 cluster build time via QEMU] → native cross-compile in builder stage, no emulated Go compile.
- [Loss of OLED/Pi provisioning] → accepted per user decision; old code remains in git history.
- [Removed config keys] → the optional config-file loader rejects unknown keys, so an old env file containing `SOIL_PI_HAT_ENABLED` fails with a diagnostic; process environment variables are not validated, so the Kubernetes path is unaffected.
- [Private GHCR package pull failures] → document pull secret or public visibility.
- [Old Pi data uses `0750 soilmonitor` ownership] → UID differs in container; the copy procedure sets ownership to 65532.

## Migration Plan

1. Merge change; CI publishes the first image.
2. In the GitOps repo, add an overlay with the broker address and image tag; the controller creates the PVC and Deployment.
3. Optionally scale to 0, copy the old SQLite file into the PVC, chown 65532, scale to 1.
4. Repoint the publishing firmware's MQTT broker to the in-cluster broker if it changes.
5. Rollback: set the previous image tag in GitOps; the database is forward-only per app migrations, so snapshot the PVC before upgrades that change schema.
