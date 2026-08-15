## 1. Define the mise workflow

- [x] 1.1 Add described `build`, `build-pi`, `test`, `fmt`, and `vet` tasks to `.mise.toml`, preserving the Makefile commands, environment variables, flags, exclusions, and output paths.
- [x] 1.2 Add a described default task that displays the native `mise` task listing without running a modifying workflow.
- [x] 1.3 Run the default task and verify that every supported task and description is displayed without changing project files.

## 2. Migrate the documented interface

- [x] 2.1 Replace Make-based developer commands in `README.md` with their `mise run` equivalents and document the default task-discovery command.
- [x] 2.2 Search the maintained repository files for remaining Makefile rule references and migrate any that describe the supported developer workflow.
- [x] 2.3 Remove the Makefile after confirming that every rule has an equivalent `mise` task.

## 3. Verify behavior parity

- [x] 3.1 Run the test and vet tasks and confirm that both complete successfully.
- [x] 3.2 Run the format task and confirm that applicable Go files are formatted while files beneath `openspec` remain untouched.
- [x] 3.3 Run the local build task and confirm that it creates `build/soil-monitor`.
- [x] 3.4 Run the Raspberry Pi build task and confirm that it creates a static Linux ARM64 binary at `build/soil-monitor-linux-arm64` with trimmed paths.
