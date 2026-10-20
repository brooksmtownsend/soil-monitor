# mise-task-workflow Specification

## Purpose
Define the supported mise tasks for building, testing, formatting, vetting, and building the container image of the soil monitor.
## Requirements

### Requirement: Discoverable default task
The project SHALL define a default `mise` task that displays the available project tasks and their descriptions without building, testing, formatting, or otherwise modifying project files.

#### Scenario: Run the default task
- **WHEN** a contributor runs `mise run` without a task name
- **THEN** the command lists the supported project tasks with descriptions
- **AND** no build, test, format, or vet workflow is executed

### Requirement: Local build task
The project SHALL provide a `mise` task that builds the soil monitor application for the local development platform at `build/soil-monitor`.

#### Scenario: Build for the local platform
- **WHEN** a contributor runs `mise run build`
- **THEN** Go builds `./cmd/soil-monitor`
- **AND** the resulting binary is written to `build/soil-monitor`

### Requirement: Raspberry Pi build task
The project SHALL provide a `mise` task that cross-compiles a static Linux ARM64 soil monitor binary at `build/soil-monitor-linux-arm64` with trimmed source paths.

#### Scenario: Cross-build for the Raspberry Pi
- **WHEN** a contributor runs `mise run build-pi`
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=arm64`, and `CGO_ENABLED=0`
- **AND** the build uses `-trimpath`
- **AND** the resulting binary is written to `build/soil-monitor-linux-arm64`

### Requirement: Test task
The project SHALL provide a `mise` task that runs the complete Go test suite.

#### Scenario: Run tests
- **WHEN** a contributor runs `mise run test`
- **THEN** Go tests all packages beneath the module
- **AND** the task exits unsuccessfully when any test fails

### Requirement: Format task
The project SHALL provide a `mise` task that formats the repository's Go source files while excluding Go files under `openspec`.

#### Scenario: Format source files
- **WHEN** a contributor runs `mise run fmt`
- **THEN** `gofmt` updates all applicable `.go` files in the repository
- **AND** `.go` files beneath `openspec` are not modified

### Requirement: Vet task
The project SHALL provide a `mise` task that runs Go static analysis across all module packages.

#### Scenario: Run static analysis
- **WHEN** a contributor runs `mise run vet`
- **THEN** `go vet` analyzes all packages beneath the module
- **AND** the task exits unsuccessfully when vet reports an error

### Requirement: Single task-runner interface
The project SHALL document `mise run` as the supported interface for routine development workflows and SHALL NOT retain the Makefile after equivalent `mise` tasks are available.

#### Scenario: Follow the documented workflow
- **WHEN** a contributor follows the repository build and test documentation
- **THEN** the documented commands invoke the corresponding `mise` tasks
- **AND** no Make installation or Makefile is required

### Requirement: Supported static Linux build tasks
The project SHALL provide discoverable mise build tasks for static Linux ARM64 and AMD64 soil-monitor binaries with trimmed source paths. The existing `build-pi` workflow SHALL remain available as the Raspberry Pi-compatible ARM64 entry point, and these artifacts SHALL be the inputs to the container image build.

#### Scenario: Build Linux ARM64 artifact
- **WHEN** a contributor runs the documented Linux ARM64 build task or `build-pi`
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=arm64`, and `CGO_ENABLED=0` and writes the documented ARM64 artifact

#### Scenario: Build Linux AMD64 artifact
- **WHEN** a contributor runs the documented Linux AMD64 build task
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0` and writes the documented AMD64 artifact

### Requirement: Container image build task
The project SHALL provide a discoverable `mise` task that builds the container image locally without pushing it.

#### Scenario: Contributor lists tasks
- **WHEN** a contributor runs `mise run`
- **THEN** the image build task is listed with a description
