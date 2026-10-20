# Spec Delta

## MODIFIED Requirements

### Requirement: Supported static Linux build tasks
The project SHALL provide discoverable mise build tasks for static Linux ARM64 and AMD64 soil-monitor binaries with trimmed source paths. The existing `build-pi` workflow SHALL remain available as the Raspberry Pi-compatible ARM64 entry point, and these artifacts SHALL be the inputs to the container image build.

#### Scenario: Build Linux ARM64 artifact
- **WHEN** a contributor runs the documented Linux ARM64 build task or `build-pi`
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=arm64`, and `CGO_ENABLED=0` and writes the documented ARM64 artifact

#### Scenario: Build Linux AMD64 artifact
- **WHEN** a contributor runs the documented Linux AMD64 build task
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0` and writes the documented AMD64 artifact

## ADDED Requirements

### Requirement: Container image build task
The project SHALL provide a discoverable `mise` task that builds the container image locally without pushing it.

#### Scenario: Contributor lists tasks
- **WHEN** a contributor runs `mise run`
- **THEN** the image build task is listed with a description

## REMOVED Requirements

### Requirement: Architecture-aware deployment tasks
**Reason**: The `release` and `bootstrap` tasks are removed.
**Migration**: Use CI image publication and the GitOps repository.
