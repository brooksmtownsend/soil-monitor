## ADDED Requirements

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
