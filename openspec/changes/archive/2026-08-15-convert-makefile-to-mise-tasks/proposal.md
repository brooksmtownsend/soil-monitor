## Why

The project currently pins Go with `mise` but exposes its development workflows through a separate Makefile, requiring contributors to understand and invoke two overlapping tool layers. Moving those workflows into `mise` creates one discoverable entry point for tool provisioning and common project commands.

## What Changes

- Define `mise` tasks for the current local build, Raspberry Pi cross-build, test, format, and vet workflows.
- Add a default `mise` task that displays the available project tasks and their descriptions.
- Update developer documentation to use the new `mise` task commands.
- Remove the Makefile after its workflows have equivalent `mise` tasks.
- **BREAKING**: Existing `make build`, `make build-pi`, `make test`, `make fmt`, and `make vet` commands will no longer be supported; their `mise run` equivalents become the documented interface.

## Capabilities

### New Capabilities

- `mise-task-workflow`: Provides discoverable, pinned development tasks for building, cross-compiling, testing, formatting, and vetting the project.

### Modified Capabilities

None.

## Impact

- Replaces the repository `Makefile` with task definitions in `.mise.toml`.
- Updates command examples in `README.md` and any other repository documentation that invokes Make.
- Does not change the application binary, MQTT contract, hardware behavior, runtime dependencies, or deployment interface.
- Requires contributors to have `mise`, which is already the documented tool bootstrap mechanism.
