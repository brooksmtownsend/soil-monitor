## Context

The repository currently uses `.mise.toml` only to pin Go 1.25 while a Makefile defines five development workflows: local build, Linux ARM64 cross-build, tests, formatting, and vetting. Documentation consequently wraps Make invocations in `mise exec`, even though `mise` can define and describe repository-local tasks directly.

The change affects contributor tooling and documentation only. The commands executed by each workflow, output paths, target architecture, and resulting application behavior must remain unchanged.

## Goals / Non-Goals

**Goals:**

- Make `mise` the single project interface for tool provisioning and routine development commands.
- Preserve the behavior and outputs of every current Make rule.
- Make the task catalog discoverable through the default `mise run` invocation.
- Keep task definitions concise, documented, and portable across supported development machines.

**Non-Goals:**

- Changing the pinned Go version or adding new development tools.
- Changing build flags, binary names, output directories, test coverage, or formatting scope.
- Changing Raspberry Pi deployment, application runtime behavior, or CI/release automation.
- Retaining compatibility aliases for the removed Make commands.

## Decisions

### Define tasks inline in `.mise.toml`

Each workflow will be represented by a named `[tasks.<name>]` entry with a description and command. Keeping the small task set in `.mise.toml` places runtime and task configuration together and avoids introducing a separate task directory. Separate task scripts were considered, but would add indirection without providing value for these short commands.

### Preserve the existing task names and commands

The task names will remain `build`, `build-pi`, `test`, `fmt`, and `vet`, now invoked with `mise run <task>`. Their underlying Go commands and environment variables will match the Makefile so the local artifacts and Linux ARM64 artifact remain compatible. Renaming tasks was considered but rejected because the current names are clear and migration is easier when only the command prefix changes.

### Use a default task for discovery

A `default` task will list available tasks through `mise`'s own task-listing facility. Every user-facing task will have a description so the output functions as concise help and remains synchronized with the actual configuration. Hard-coding help text was considered but rejected because it can drift when tasks change.

### Remove the Makefile in the same change

The Makefile will be deleted once parity is verified, leaving a single supported task interface. Keeping a forwarding Makefile was considered, but would preserve the duplicated interface this change is intended to eliminate.

### Update all documented invocations

Repository documentation will replace `mise exec -- make <rule>` and direct `make <rule>` examples with `mise run <task>`. Documentation will also show the default help behavior so a contributor can discover commands without reading `.mise.toml`.

## Risks / Trade-offs

- [Task behavior diverges from the Makefile during translation] → Copy the existing commands and environment values exactly, then run every task and compare expected output locations.
- [The default task includes itself or produces unclear help] → Use the native `mise tasks` listing and provide descriptions for all tasks; verify the rendered task list manually.
- [Formatting scans generated or cached Go files] → Preserve the Makefile's existing `find` exclusions rather than broadening the formatting command.
- [Removing Make disrupts existing personal workflows] → Document the one-to-one command mapping in the README; no compatibility layer is retained because consolidating on `mise` is an explicit goal.
