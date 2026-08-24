#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "${BASH_SOURCE[0]%/*}" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
readonly REPO_ROOT

readonly TARGET="${SOIL_RELEASE_TARGET-brooks@192.168.1.95}"
readonly BINARY_OVERRIDE="${SOIL_RELEASE_BINARY-}"
readonly LOCAL_BINARY_ARM64="${SOIL_RELEASE_BINARY_ARM64-${BINARY_OVERRIDE:-$REPO_ROOT/build/soil-monitor-linux-arm64}}"
readonly LOCAL_BINARY_AMD64="${SOIL_RELEASE_BINARY_AMD64-${BINARY_OVERRIDE:-$REPO_ROOT/build/soil-monitor-linux-amd64}}"
readonly LOCAL_UNIT="${SOIL_RELEASE_UNIT-$REPO_ROOT/deploy/soil-monitor.service}"
readonly REMOTE_SCRIPT="$SCRIPT_DIR/release-remote.sh"
readonly REMOTE_BINARY="${SOIL_RELEASE_REMOTE_BINARY-/usr/local/bin/soil-monitor}"
readonly REMOTE_UNIT="${SOIL_RELEASE_REMOTE_UNIT-/etc/systemd/system/soil-monitor.service}"
readonly REMOTE_ENV="${SOIL_RELEASE_REMOTE_ENV-/etc/soil-monitor/soil-monitor.env}"
readonly REMOTE_STAGE="${SOIL_RELEASE_STAGE_DIR-/tmp/soil-monitor-release}"
readonly SERVICE="${SOIL_RELEASE_SERVICE-soil-monitor.service}"
readonly SERVICE_USER="${SOIL_RELEASE_SERVICE_USER-soilmonitor}"
readonly VERIFY_ATTEMPTS="${SOIL_RELEASE_VERIFY_ATTEMPTS-10}"
readonly VERIFY_DELAY="${SOIL_RELEASE_VERIFY_DELAY_SECONDS-1}"
readonly DRY_RUN="${SOIL_RELEASE_DRY_RUN-0}"

die() {
  printf 'release error: %s\n' "$*" >&2
  exit 1
}

validate_target() {
  [[ "$1" =~ ^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+$ ]] ||
    die "SOIL_RELEASE_TARGET must use user@host syntax with no shell metacharacters"
}

validate_account() {
  [[ "$1" =~ ^[A-Za-z_][A-Za-z0-9_-]*$ ]] ||
    die "SOIL_RELEASE_SERVICE_USER must be a valid local account name"
}

validate_absolute_path() {
  local variable_name=$1
  local value=$2

  [[ "$value" =~ ^/[A-Za-z0-9._/-]+$ ]] ||
    die "$variable_name must be a non-empty absolute path containing only letters, numbers, '.', '_', '-', and '/'"
  [[ "$value" != "/" && "$value/" != *"/../"* && "$value/" != *"/./"* && "$value" != *"//"* ]] ||
    die "$variable_name must not be '/', contain repeated separators, or contain '.' or '..' path segments"
}

validate_positive_integer() {
  local variable_name=$1
  local value=$2

  [[ "$value" =~ ^[1-9][0-9]*$ ]] || die "$variable_name must be a positive integer"
}

validate_nonnegative_integer() {
  local variable_name=$1
  local value=$2

  [[ "$value" =~ ^[0-9]+$ ]] || die "$variable_name must be a non-negative integer"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 ||
    die "required local command '$1' was not found; install it and retry"
}

validate_target "$TARGET"
validate_absolute_path SOIL_RELEASE_BINARY_ARM64 "$LOCAL_BINARY_ARM64"
validate_absolute_path SOIL_RELEASE_BINARY_AMD64 "$LOCAL_BINARY_AMD64"
validate_absolute_path SOIL_RELEASE_UNIT "$LOCAL_UNIT"
validate_absolute_path SOIL_RELEASE_REMOTE_BINARY "$REMOTE_BINARY"
validate_absolute_path SOIL_RELEASE_REMOTE_UNIT "$REMOTE_UNIT"
validate_absolute_path SOIL_RELEASE_REMOTE_ENV "$REMOTE_ENV"
validate_absolute_path SOIL_RELEASE_STAGE_DIR "$REMOTE_STAGE"
validate_account "$SERVICE_USER"
[[ "$SERVICE" =~ ^[A-Za-z0-9_.@-]+\.service$ ]] ||
  die "SOIL_RELEASE_SERVICE must be a valid systemd .service unit name"
validate_positive_integer SOIL_RELEASE_VERIFY_ATTEMPTS "$VERIFY_ATTEMPTS"
validate_nonnegative_integer SOIL_RELEASE_VERIFY_DELAY_SECONDS "$VERIFY_DELAY"
[[ "$DRY_RUN" == "0" || "$DRY_RUN" == "1" ]] || die "SOIL_RELEASE_DRY_RUN must be 0 or 1"

require_command ssh
require_command scp
[[ -f "$LOCAL_BINARY_ARM64" && -x "$LOCAL_BINARY_ARM64" ]] ||
  die "Linux ARM64 binary is missing or not executable at $LOCAL_BINARY_ARM64; run 'mise run build-linux-arm64'"
[[ -f "$LOCAL_BINARY_AMD64" && -x "$LOCAL_BINARY_AMD64" ]] ||
  die "Linux AMD64 binary is missing or not executable at $LOCAL_BINARY_AMD64; run 'mise run build-linux-amd64'"
[[ -f "$LOCAL_UNIT" ]] || die "systemd unit is missing at $LOCAL_UNIT"
[[ -f "$REMOTE_SCRIPT" ]] || die "remote release helper is missing at $REMOTE_SCRIPT"

printf 'Release plan\n'
printf '  Target:             %s\n' "$TARGET"
printf '  ARM64 artifact:     %s\n' "$LOCAL_BINARY_ARM64"
printf '  AMD64 artifact:     %s\n' "$LOCAL_BINARY_AMD64"
printf '  Target architecture: detected remotely (ARM64 or AMD64)\n'
printf '  Unit artifact:      %s\n' "$LOCAL_UNIT"
printf '  Staging directory:  %s\n' "$REMOTE_STAGE"
printf '  Binary destination: %s\n' "$REMOTE_BINARY"
printf '  Unit destination:   %s\n' "$REMOTE_UNIT"
printf '  Environment file:   %s (preserved)\n' "$REMOTE_ENV"
printf '  Service account:    %s\n' "$SERVICE_USER"
printf '  Service:            %s\n' "$SERVICE"
printf '  Service actions:    validate unit, daemon-reload, restart, verify active\n'

if [[ "$DRY_RUN" == "1" ]]; then
  printf 'Dry run complete: SSH and artifact transfer were skipped.\n'
  exit 0
fi

readonly -a SSH_OPTIONS=(-o BatchMode=yes -o ConnectTimeout=10)

printf 'Checking SSH connectivity to %s...\n' "$TARGET"
if ! ssh "${SSH_OPTIONS[@]}" "$TARGET" true; then
  die "batch-mode SSH failed for $TARGET; install the SSH key and verify the host is reachable"
fi

printf 'Detecting remote architecture...\n'
remote_arch=$(ssh "${SSH_OPTIONS[@]}" "$TARGET" uname -m) ||
  die "could not determine the remote architecture"
case "$(printf '%s' "$remote_arch" | tr -d '[:space:]')" in
  aarch64|arm64)
    readonly LOCAL_BINARY="$LOCAL_BINARY_ARM64"
    readonly GOARCH=arm64
    ;;
  x86_64|amd64)
    readonly LOCAL_BINARY="$LOCAL_BINARY_AMD64"
    readonly GOARCH=amd64
    ;;
  *) die "unsupported remote architecture: $remote_arch (supported: ARM64 and AMD64)" ;;
esac
printf 'Selected Linux %s artifact: %s\n' "$GOARCH" "$LOCAL_BINARY"

printf 'Checking remote release prerequisites...\n'
if ! ssh "${SSH_OPTIONS[@]}" "$TARGET" sh -s -- \
  preflight "$REMOTE_STAGE" "$REMOTE_BINARY" "$REMOTE_UNIT" "$REMOTE_ENV" "$SERVICE" "$SERVICE_USER" \
  <"$REMOTE_SCRIPT"; then
  die "remote preflight failed; review the diagnostic above and complete one-time host setup"
fi

cleanup_stage() {
  ssh "${SSH_OPTIONS[@]}" "$TARGET" sh -s -- cleanup "$REMOTE_STAGE" \
    <"$REMOTE_SCRIPT" >/dev/null 2>&1 || true
}

printf 'Transferring release artifacts...\n'
if ! scp "${SSH_OPTIONS[@]}" -- "$LOCAL_BINARY" "$TARGET:$REMOTE_STAGE/soil-monitor"; then
  cleanup_stage
  die "binary transfer failed; remote installation was not attempted"
fi
if ! scp "${SSH_OPTIONS[@]}" -- "$LOCAL_UNIT" "$TARGET:$REMOTE_STAGE/soil-monitor.service"; then
  cleanup_stage
  die "unit transfer failed; remote installation was not attempted"
fi

printf 'Installing and activating %s...\n' "$SERVICE"
if ! ssh "${SSH_OPTIONS[@]}" "$TARGET" sh -s -- \
  install "$REMOTE_STAGE" "$REMOTE_BINARY" "$REMOTE_UNIT" "$REMOTE_ENV" "$SERVICE" \
  "$SERVICE_USER" "$VERIFY_ATTEMPTS" "$VERIFY_DELAY" <"$REMOTE_SCRIPT"; then
  die "release activation failed; review the remote diagnostics and rollback result above"
fi

printf 'Release successful: %s is active on %s.\n' "$SERVICE" "$TARGET"
