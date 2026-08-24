# mise-task-workflow Specification

## Purpose
TBD - created by archiving change convert-makefile-to-mise-tasks. Update Purpose after archive.
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
The project SHALL provide discoverable mise build tasks for static Linux ARM64 and AMD64 soil-monitor binaries with trimmed source paths. The existing `build-pi` workflow SHALL remain available as the Raspberry Pi-compatible ARM64 entry point.

#### Scenario: Build Linux ARM64 artifact
- **WHEN** a contributor runs the documented Linux ARM64 build task or `build-pi`
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=arm64`, and `CGO_ENABLED=0` and writes the documented ARM64 artifact

#### Scenario: Build Linux AMD64 artifact
- **WHEN** a contributor runs the documented Linux AMD64 build task
- **THEN** Go builds `./cmd/soil-monitor` with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0` and writes the documented AMD64 artifact

### Requirement: Architecture-aware deployment tasks
The `release` and `bootstrap` mise tasks SHALL validate and build every supported Linux deployment artifact required for remote architecture selection before transferring or changing remote files, and their descriptions SHALL describe supported Debian targets rather than only Raspberry Pi targets.

#### Scenario: Contributor lists tasks
- **WHEN** a contributor runs `mise run`
- **THEN** the build, release, and bootstrap descriptions identify the supported Linux/Debian workflows and the Raspberry Pi-compatible path

#### Scenario: Deployment build fails
- **WHEN** either supported deployment artifact cannot be built
- **THEN** release or bootstrap exits before any remote file is transferred or changed

