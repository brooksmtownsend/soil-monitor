#!/bin/sh
set -eu

die() {
  printf 'remote release error: %s\n' "$*" >&2
  exit 1
}

validate_absolute_path() {
  variable_name=$1
  value=$2

  case "$value" in
    /*) ;;
    *) die "$variable_name must be an absolute path" ;;
  esac
  case "$value/" in
    *'/../'*|*'/./'*|*'//'*) die "$variable_name contains an unsafe path segment" ;;
  esac
  [ "$value" != "/" ] || die "$variable_name must not be /"
  case "$value" in
    *[!A-Za-z0-9._/-]*) die "$variable_name contains unsupported characters" ;;
  esac
}

validate_service() {
  case "$1" in
    ''|*[!A-Za-z0-9_.@-]*|*.service.service) die "invalid systemd service name" ;;
    *.service) ;;
    *) die "service name must end in .service" ;;
  esac
}

validate_account() {
  case "$1" in
    ''|*[!A-Za-z0-9_-]*|[0-9-]*) die "invalid service account name" ;;
  esac
}

require_remote_command() {
  command -v "$1" >/dev/null 2>&1 || die "required remote command '$1' was not found"
}

mode=${1-}
[ -n "$mode" ] || die "operation mode is required"
shift

case "$mode" in
  cleanup)
    [ "$#" -eq 1 ] || die "cleanup expects the staging path"
    stage=$1
    validate_absolute_path SOIL_RELEASE_STAGE_DIR "$stage"
    rm -rf -- "$stage"
    ;;

  preflight)
    [ "$#" -eq 6 ] || die "preflight received an invalid argument count"
    stage=$1
    binary=$2
    unit=$3
    environment=$4
    service=$5
    service_user=$6
    validate_absolute_path SOIL_RELEASE_STAGE_DIR "$stage"
    validate_absolute_path SOIL_RELEASE_REMOTE_BINARY "$binary"
    validate_absolute_path SOIL_RELEASE_REMOTE_UNIT "$unit"
    validate_absolute_path SOIL_RELEASE_REMOTE_ENV "$environment"
    validate_service "$service"
    validate_account "$service_user"

    for command_name in sudo install cp rm mkdir systemctl systemd-analyze journalctl sleep id; do
      require_remote_command "$command_name"
    done
    id "$service_user" >/dev/null 2>&1 ||
      die "service account $service_user is missing; rerun bootstrap"
    sudo -n true >/dev/null 2>&1 ||
      die "non-interactive sudo is unavailable; install the documented sudoers policy"
    sudo -n -u "$service_user" test -r "$environment" ||
      die "$environment is missing or not readable by $service_user; rerun bootstrap or fix ownership and mode"
    rm -rf -- "$stage"
    mkdir -m 700 -- "$stage" || die "cannot create staging directory $stage"
    [ -w "$stage" ] || die "staging directory $stage is not writable"
    printf 'Remote preflight passed for %s.\n' "$service"
    ;;

  install)
    [ "$#" -eq 8 ] || die "install received an invalid argument count"
    stage=$1
    binary=$2
    unit=$3
    environment=$4
    service=$5
    service_user=$6
    attempts=$7
    delay=$8
    validate_absolute_path SOIL_RELEASE_STAGE_DIR "$stage"
    validate_absolute_path SOIL_RELEASE_REMOTE_BINARY "$binary"
    validate_absolute_path SOIL_RELEASE_REMOTE_UNIT "$unit"
    validate_absolute_path SOIL_RELEASE_REMOTE_ENV "$environment"
    validate_service "$service"
    validate_account "$service_user"
    case "$attempts" in ''|*[!0-9]*|0) die "verification attempts must be positive" ;; esac
    case "$delay" in ''|*[!0-9]*) die "verification delay must be non-negative" ;; esac

    staged_binary="$stage/soil-monitor"
    staged_unit="$stage/soil-monitor.service"
    [ -f "$staged_binary" ] || die "staged binary is missing"
    [ -f "$staged_unit" ] || die "staged unit is missing"
    sudo -n -u "$service_user" test -r "$environment" ||
      die "$environment disappeared or became unreadable by $service_user before installation"
    trap 'rm -rf -- "$stage"' EXIT HUP INT TERM

    had_binary=0
    had_unit=0
    if [ -f "$binary" ]; then
      sudo -n cp -p -- "$binary" "$binary.previous"
      had_binary=1
    fi
    if [ -f "$unit" ]; then
      sudo -n cp -p -- "$unit" "$unit.previous"
      had_unit=1
    fi

    sudo -n install -o root -g root -m 0755 "$staged_binary" "$binary"
    sudo -n install -o root -g root -m 0644 "$staged_unit" "$unit"

    activation_ok=1
    if ! systemd-analyze verify "$unit"; then
      printf 'Installed systemd unit failed validation.\n' >&2
      activation_ok=0
    fi
    if [ "$activation_ok" -eq 1 ]; then
      sudo -n systemctl daemon-reload || activation_ok=0
    fi
    if [ "$activation_ok" -eq 1 ]; then
      sudo -n systemctl restart "$service" || activation_ok=0
    fi

    if [ "$activation_ok" -eq 1 ]; then
      current_attempt=1
      while [ "$current_attempt" -le "$attempts" ]; do
        if sudo -n systemctl is-active --quiet "$service"; then
          printf '%s is active.\n' "$service"
          exit 0
        fi
        if [ "$delay" -gt 0 ]; then
          sleep "$delay"
        fi
        current_attempt=$((current_attempt + 1))
      done
    fi

    printf 'New release did not become active; collecting diagnostics.\n' >&2
    sudo -n systemctl status --no-pager --full "$service" >&2 || true
    sudo -n journalctl -u "$service" -n 50 --no-pager >&2 || true

    if [ "$had_binary" -eq 1 ] && [ "$had_unit" -eq 1 ]; then
      printf 'Restoring .previous binary and unit backups...\n' >&2
      rollback_ok=1
      sudo -n install -o root -g root -m 0755 "$binary.previous" "$binary" || rollback_ok=0
      sudo -n install -o root -g root -m 0644 "$unit.previous" "$unit" || rollback_ok=0
      sudo -n systemctl daemon-reload || rollback_ok=0
      sudo -n systemctl restart "$service" || rollback_ok=0
      sudo -n systemctl is-active --quiet "$service" || rollback_ok=0
      if [ "$rollback_ok" -eq 1 ]; then
        printf 'Rollback succeeded; the previous service version is active.\n' >&2
      else
        printf 'Rollback was attempted but did not restore an active service.\n' >&2
      fi
    else
      printf 'Automatic rollback unavailable: this host did not have both prior artifacts.\n' >&2
    fi
    exit 1
    ;;

  *)
    die "unknown operation mode: $mode"
    ;;
esac
