#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "${BASH_SOURCE[0]%/*}" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
readonly REPO_ROOT

readonly TARGET="${SOIL_RELEASE_TARGET-brooks@192.168.1.95}"
readonly STAGE="${SOIL_BOOTSTRAP_STAGE_DIR-/tmp/soil-monitor-bootstrap}"
readonly SPI_PORT="${SOIL_BOOTSTRAP_SPI_PORT-/dev/spidev0.0}"
readonly DISCONNECT_ATTEMPTS="${SOIL_BOOTSTRAP_DISCONNECT_ATTEMPTS-15}"
readonly RECONNECT_ATTEMPTS="${SOIL_BOOTSTRAP_RECONNECT_ATTEMPTS-60}"
readonly WAIT_DELAY="${SOIL_BOOTSTRAP_WAIT_SECONDS-2}"
readonly DRY_RUN="${SOIL_BOOTSTRAP_DRY_RUN-0}"
PI_HAT_ENABLED="${SOIL_BOOTSTRAP_PI_HAT_ENABLED-true}"
readonly REMOTE_BOOTSTRAP_SCRIPT="${SOIL_BOOTSTRAP_REMOTE_SCRIPT-$SCRIPT_DIR/bootstrap-pi.sh}"
readonly ENVIRONMENT_EXAMPLE="${SOIL_BOOTSTRAP_ENVIRONMENT_EXAMPLE-$REPO_ROOT/deploy/soil-monitor.env.example}"
readonly MOSQUITTO_CONFIG="${SOIL_BOOTSTRAP_MOSQUITTO_CONFIG-$REPO_ROOT/deploy/soil-monitor-mosquitto.conf}"
readonly UNIT="${SOIL_BOOTSTRAP_UNIT-$REPO_ROOT/deploy/soil-monitor.service}"
readonly SUDOERS_POLICY="${SOIL_BOOTSTRAP_SUDOERS_POLICY-$REPO_ROOT/deploy/soil-monitor-release.sudoers}"
readonly RELEASE_SCRIPT="${SOIL_BOOTSTRAP_RELEASE_SCRIPT-$SCRIPT_DIR/release.sh}"

die() {
  printf 'bootstrap error: %s\n' "$*" >&2
  exit 1
}

validate_target() {
  [[ "$1" =~ ^[A-Za-z_][A-Za-z0-9_-]*@[A-Za-z0-9.-]+$ ]] ||
    die "SOIL_RELEASE_TARGET must use user@host syntax with no shell metacharacters"
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
  command -v "$1" >/dev/null 2>&1 || die "required local command '$1' was not found"
}

for argument in "$@"; do
  case "$argument" in
    --pi-hat=true) PI_HAT_ENABLED=true ;;
    --pi-hat=false) PI_HAT_ENABLED=false ;;
    --pi-hat) die "--pi-hat requires an explicit true or false value" ;;
    *) die "unknown bootstrap argument: $argument" ;;
  esac
done
readonly PI_HAT_ENABLED

validate_target "$TARGET"
validate_absolute_path SOIL_BOOTSTRAP_STAGE_DIR "$STAGE"
if [[ "$PI_HAT_ENABLED" == "true" ]]; then
  validate_absolute_path SOIL_BOOTSTRAP_SPI_PORT "$SPI_PORT"
fi
validate_positive_integer SOIL_BOOTSTRAP_DISCONNECT_ATTEMPTS "$DISCONNECT_ATTEMPTS"
validate_positive_integer SOIL_BOOTSTRAP_RECONNECT_ATTEMPTS "$RECONNECT_ATTEMPTS"
validate_nonnegative_integer SOIL_BOOTSTRAP_WAIT_SECONDS "$WAIT_DELAY"
[[ "$DRY_RUN" == "0" || "$DRY_RUN" == "1" ]] || die "SOIL_BOOTSTRAP_DRY_RUN must be 0 or 1"
[[ "$PI_HAT_ENABLED" == "true" || "$PI_HAT_ENABLED" == "false" ]] ||
  die "SOIL_BOOTSTRAP_PI_HAT_ENABLED must be true or false"

for command_name in ssh scp sleep; do
  require_command "$command_name"
done
for required_file in "$REMOTE_BOOTSTRAP_SCRIPT" "$ENVIRONMENT_EXAMPLE" "$MOSQUITTO_CONFIG" "$UNIT" "$SUDOERS_POLICY" "$RELEASE_SCRIPT"; do
  [[ -f "$required_file" ]] || die "required bootstrap asset is missing: $required_file"
done

readonly RELEASE_USER="${TARGET%@*}"
readonly -a SSH_OPTIONS=(-o BatchMode=yes -o ConnectTimeout=5)

printf 'Bootstrap plan\n'
printf '  Target:               %s\n' "$TARGET"
printf '  Remote staging:       %s\n' "$STAGE"
printf '  Initial MQTT broker:  tcp://127.0.0.1:1883\n'
printf '  MQTT LAN listener:    1883 (anonymous, trusted LAN only)\n'
printf '  Pi HAT enabled:       %s\n' "$PI_HAT_ENABLED"
printf '  Remote architecture: detected during release (ARM64 or AMD64)\n'
printf '  Existing environment: preserved\n'
if [[ "$PI_HAT_ENABLED" == "true" ]]; then
  printf '  SPI device:           %s\n' "$SPI_PORT"
  printf '  Reboot:               only if SPI is not active\n'
else
  printf '  Hardware setup:       omitted for generic Debian mode\n'
fi

if [[ "$DRY_RUN" == "1" ]]; then
  printf 'Dry run complete: SSH, provisioning, reboot, and release were skipped.\n'
  exit 0
fi

printf 'Checking SSH connectivity to %s...\n' "$TARGET"
ssh "${SSH_OPTIONS[@]}" "$TARGET" true ||
  die "batch-mode SSH failed; verify the fresh image's address, host key, and authorized SSH key"

cleanup_stage() {
  ssh "${SSH_OPTIONS[@]}" "$TARGET" rm -rf -- "$STAGE" >/dev/null 2>&1 || true
}
trap cleanup_stage EXIT

printf 'Creating remote bootstrap staging directory...\n'
ssh "${SSH_OPTIONS[@]}" "$TARGET" mkdir -p -- "$STAGE" || die "could not create remote bootstrap staging directory"
ssh "${SSH_OPTIONS[@]}" "$TARGET" chmod 700 -- "$STAGE" || die "could not secure remote bootstrap staging directory"

printf 'Transferring bootstrap assets...\n'
scp "${SSH_OPTIONS[@]}" -- "$REMOTE_BOOTSTRAP_SCRIPT" "$TARGET:$STAGE/bootstrap-pi.sh" || die "could not stage bootstrap script"
scp "${SSH_OPTIONS[@]}" -- "$ENVIRONMENT_EXAMPLE" "$TARGET:$STAGE/soil-monitor.env.example" || die "could not stage environment example"
scp "${SSH_OPTIONS[@]}" -- "$MOSQUITTO_CONFIG" "$TARGET:$STAGE/soil-monitor-mosquitto.conf" || die "could not stage Mosquitto configuration"
scp "${SSH_OPTIONS[@]}" -- "$UNIT" "$TARGET:$STAGE/soil-monitor.service" || die "could not stage systemd unit"
scp "${SSH_OPTIONS[@]}" -- "$SUDOERS_POLICY" "$TARGET:$STAGE/soil-monitor-release.sudoers" || die "could not stage release sudo policy"

printf 'Running privileged Debian host bootstrap; sudo may prompt once...\n'
ssh -tt "${SSH_OPTIONS[@]}" "$TARGET" sudo sh "$STAGE/bootstrap-pi.sh" "$STAGE" "$RELEASE_USER" "$PI_HAT_ENABLED" ||
  die "remote bootstrap failed; review its last diagnostic and rerun bootstrap after correcting it"

if [[ "$PI_HAT_ENABLED" == "true" ]]; then
  if ssh "${SSH_OPTIONS[@]}" "$TARGET" test -e "$SPI_PORT"; then
    printf 'SPI is already active; reboot skipped.\n'
  else
    printf 'SPI is not active in the running kernel; rebooting %s...\n' "$TARGET"
    ssh -tt "${SSH_OPTIONS[@]}" "$TARGET" sudo systemctl reboot || true

    disconnected=0
    for ((attempt = 1; attempt <= DISCONNECT_ATTEMPTS; attempt++)); do
      if ! ssh "${SSH_OPTIONS[@]}" "$TARGET" true >/dev/null 2>&1; then
        disconnected=1
        break
      fi
      sleep "$WAIT_DELAY"
    done
    [[ "$disconnected" -eq 1 ]] || die "the Pi did not disconnect for reboot within the configured timeout"

    reconnected=0
    for ((attempt = 1; attempt <= RECONNECT_ATTEMPTS; attempt++)); do
      if ssh "${SSH_OPTIONS[@]}" "$TARGET" true >/dev/null 2>&1; then
        reconnected=1
        break
      fi
      sleep "$WAIT_DELAY"
    done
    [[ "$reconnected" -eq 1 ]] || die "the Pi did not return over SSH after reboot; inspect Wi-Fi or the image locally"
  fi

  ssh "${SSH_OPTIONS[@]}" "$TARGET" test -e "$SPI_PORT" ||
    die "SPI device $SPI_PORT is still missing after bootstrap and reboot"
fi

printf 'Running the existing guarded application release...\n'
SOIL_RELEASE_TARGET="$TARGET" "$RELEASE_SCRIPT" || die "application release failed after host bootstrap"

ssh "${SSH_OPTIONS[@]}" "$TARGET" systemctl is-active --quiet mosquitto.service ||
  die "mosquitto.service is not active after bootstrap"
ssh "${SSH_OPTIONS[@]}" "$TARGET" systemctl is-active --quiet soil-monitor.service ||
  die "soil-monitor.service is not active after bootstrap"

printf 'Bootstrap successful: %s is running the soil monitor and local MQTT broker.\n' "$TARGET"
