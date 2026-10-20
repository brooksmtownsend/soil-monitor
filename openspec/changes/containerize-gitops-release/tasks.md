# Tasks

## 1. Remove OLED/HAT support

- [x] 1.1 Delete `internal/hardware/` and `internal/display/` and their tests, remove hardware initialization from `cmd/soil-monitor/main.go`, and verify `go build ./...` succeeds
- [x] 1.2 Remove the `Display`/`ButtonSource`/`Button` ports, `View`, `Previous`/`Next`, selection state, and `Snapshot.SelectedPlantID` from `internal/core`, simplify `app.Runner`, drop the web template `selected` class, and update tests; verify `go test ./...` and `go test -race ./...` pass
- [x] 1.3 Remove `SOIL_PI_HAT_ENABLED`, `SOIL_SPI_PORT`, `SOIL_BUTTON_DEBOUNCE_MS`, their flags, defaults, and help text from `internal/config`; add a test that removed keys are rejected; verify config tests pass
- [x] 1.4 Run `go mod tidy` and verify `periph.io/*` and `golang.org/x/image` no longer appear in `go.mod`, and `mise run vet` passes

## 2. Container image

- [x] 2.1 Add multi-stage `Dockerfile` (cross-compiling builder → distroless static nonroot, `/data` owned by 65532, HTTP/database env defaults, binary as entrypoint) and `.dockerignore`; verify `docker build .` succeeds and `docker run` with only `SOIL_MQTT_BROKER` serves the dashboard on 8080 with `--read-only --cap-drop=ALL --user 65532` and a mounted `/data`
- [x] 2.2 Verify graceful shutdown: `docker stop` exits 0 promptly and the database is intact after restart
- [x] 2.3 Build for `linux/amd64` and `linux/arm64` with `docker buildx` and verify both manifests exist and `docker image inspect` shows non-root user and no shell
- [x] 2.4 Add a `mise` image build task (local, no push) and verify `mise run` lists it and it produces the image

## 3. GitHub Actions publishing

- [ ] 3.1 Add `.github/workflows/image.yml` with test job (vet + tests), PR build-without-push, SHA/semver tags to `ghcr.io`, least-privilege permissions, provenance and OCI labels; verify with `actionlint` and a PR run that builds without pushing
- [ ] 3.2 Verify a default-branch push publishes a `sha-*` multi-arch image and a `vX.Y.Z` tag publishes `X.Y.Z` (`docker buildx imagetools inspect`)

## 4. Kubernetes base

- [x] 4.1 Add `deploy/kubernetes/` Kustomize base (Deployment with `replicas: 1`/`Recreate`, hardened securityContext, probes, env, Service 80→8080, PVC); verify `kustomize build deploy/kubernetes` renders and `kubectl apply --dry-run=server` (or `kubeconform`) passes
- [ ] 4.2 Verify on a test cluster (kind/k3d or the real one): pod becomes Ready, history persists across pod deletion and an image tag change
- [ ] 4.3 Document overlay usage, image pull secret/public package, PVC storage class, and Pi database import in `deploy/kubernetes/README.md`; verify commands run as written

## 5. Remove legacy host tooling

- [x] 5.1 Delete `scripts/`, systemd/sudoers/env/Mosquitto assets under `deploy/`, and the `bootstrap`, `release`, and `lint-shell` mise tasks; verify `mise run` lists only remaining tasks and `mise run test` and `mise run vet` pass
- [x] 5.2 Update `README.md` and `AGENTS.md` (repo map, supported commands, deployment boundaries, remove dry-run/sudo/SSH/OLED/HAT/SPI guidance), and update the `Purpose` text of the `plant-moisture-display` and `runtime-configuration` main specs after sync; verify no references to bootstrap/release/systemd/OLED remain via `grep`
- [x] 5.3 Run `go test -race ./...`, `openspec validate containerize-gitops-release --strict`, sync deltas into `openspec/specs/`, and verify specs for removed capabilities are gone
