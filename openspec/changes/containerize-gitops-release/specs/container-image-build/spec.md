# Spec Delta

## Purpose

Define the contents and runtime contract of the soil-monitor container image so it can run unprivileged and headless on Linux ARM64 and AMD64 Kubernetes nodes.

## ADDED Requirements

### Requirement: Minimal static image
The project SHALL provide a container image that contains the statically linked soil-monitor binary and only the files the application needs at runtime (such as CA certificates), with no shell, package manager, or build toolchain.

#### Scenario: Image contents are inspected
- **WHEN** the built image's filesystem is listed
- **THEN** it contains the soil-monitor binary and no compiler, package manager, or shell

### Requirement: Multi-architecture image
The image SHALL be published for both `linux/arm64` and `linux/amd64` under a single multi-platform tag.

#### Scenario: Pull on either architecture
- **WHEN** a node of either supported architecture pulls a published tag
- **THEN** it receives a matching-architecture image that starts the application

### Requirement: Unprivileged runtime
The image SHALL run as a fixed non-root numeric user and group, and the application MUST function with all Linux capabilities dropped and a read-only root filesystem when a writable data directory is mounted.

#### Scenario: Container starts with restricted security context
- **WHEN** the container runs non-root, read-only root filesystem, no added capabilities, with a writable volume at the data directory
- **THEN** the application starts, opens its database, and serves the dashboard

### Requirement: Container defaults
The image SHALL default to `SOIL_HTTP_ENABLED=true`, an unprivileged `SOIL_HTTP_ADDR` (`:8080`), and `SOIL_DATABASE_PATH` inside a dedicated data directory. All settings SHALL remain overridable through environment variables per runtime configuration.

#### Scenario: Image started with only a broker address
- **WHEN** the container is started with only `SOIL_MQTT_BROKER` set
- **THEN** the dashboard listens on port 8080, and history is written under the data directory

### Requirement: Graceful termination
The container's entrypoint SHALL be the application binary itself so that a container-runtime termination signal reaches the application directly and triggers graceful shutdown.

#### Scenario: Pod is terminated
- **WHEN** the runtime sends SIGTERM to the container
- **THEN** the application closes the HTTP listener and database and exits successfully

### Requirement: Image build task
The project SHALL provide a documented `mise` task that builds the image locally for the host platform without pushing it.

#### Scenario: Contributor builds locally
- **WHEN** a contributor runs the image build task
- **THEN** a local image tagged for the project is produced and nothing is pushed to a registry
