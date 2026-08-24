#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "${BASH_SOURCE[0]%/*}" && pwd)"
readonly SCRIPT_DIR
readonly BOOTSTRAP_SCRIPT="$SCRIPT_DIR/bootstrap.sh"
readonly PI_SCRIPT="$SCRIPT_DIR/bootstrap-pi.sh"

pass_count=0
fail_count=0

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local value=$1
  local expected=$2
  [[ "$value" == *"$expected"* ]] || fail "expected output to contain: $expected"
}

assert_file_contains() {
  local path=$1
  local expected=$2
  grep -F -- "$expected" "$path" >/dev/null || fail "expected $path to contain: $expected"
}

run_test() {
  local name=$1
  shift
  if (set -e; "$@"); then
    printf 'ok - %s\n' "$name"
    pass_count=$((pass_count + 1))
  else
    printf 'not ok - %s\n' "$name" >&2
    fail_count=$((fail_count + 1))
  fi
}

make_local_fixture() {
  TEST_ROOT=$(mktemp -d)
  export TEST_ROOT
  trap 'rm -rf -- "$TEST_ROOT"' EXIT
  mkdir -p "$TEST_ROOT/bin"
  export SOIL_TEST_LOG="$TEST_ROOT/commands.log"
  export SOIL_TEST_STATE="$TEST_ROOT/state"
  export SOIL_RELEASE_TARGET=alice@pi.example
  export SOIL_BOOTSTRAP_STAGE_DIR=/tmp/test-soil-bootstrap
  export SOIL_BOOTSTRAP_WAIT_SECONDS=0
  export SOIL_BOOTSTRAP_DISCONNECT_ATTEMPTS=2
  export SOIL_BOOTSTRAP_RECONNECT_ATTEMPTS=2
  export SOIL_BOOTSTRAP_DRY_RUN=0
  export SOIL_BOOTSTRAP_RELEASE_SCRIPT="$TEST_ROOT/fake-release"
  unset SOIL_BOOTSTRAP_PI_HAT_ENABLED SOIL_FAKE_ARCH

  cat >"$TEST_ROOT/bin/ssh" <<'FAKE_SSH'
#!/bin/sh
set -eu
printf 'SSH %s\n' "$*" >>"$SOIL_TEST_LOG"
case " $* " in
  *' sudo sh '*bootstrap-pi.sh*)
    [ "${SOIL_FAKE_BOOTSTRAP_FAIL-0}" = "0" ] || exit 73
    ;;
  *' sudo systemctl reboot '*)
    : >"$SOIL_TEST_STATE.rebooted"
    ;;
  *' test -e /dev/spidev0.0 '*)
    if [ "${SOIL_FAKE_SPI_READY_INITIAL-0}" = "1" ] || [ -f "$SOIL_TEST_STATE.rebooted" ]; then
      exit 0
    fi
    exit 1
    ;;
  *' systemctl is-active --quiet mosquitto.service '*|*' systemctl is-active --quiet soil-monitor.service '*)
    exit 0
    ;;
  *' true '*)
    if [ ! -f "$SOIL_TEST_STATE.connected" ]; then
      : >"$SOIL_TEST_STATE.connected"
      [ "${SOIL_FAKE_INITIAL_SSH_FAIL-0}" = "0" ] || exit 72
    elif [ -f "$SOIL_TEST_STATE.rebooted" ] && [ ! -f "$SOIL_TEST_STATE.disconnected" ]; then
      : >"$SOIL_TEST_STATE.disconnected"
      exit 1
    fi
    ;;
esac
FAKE_SSH

  cat >"$TEST_ROOT/bin/scp" <<'FAKE_SCP'
#!/bin/sh
set -eu
printf 'SCP %s\n' "$*" >>"$SOIL_TEST_LOG"
[ "${SOIL_FAKE_SCP_FAIL-0}" = "0" ] || exit 74
FAKE_SCP

  cat >"$TEST_ROOT/fake-release" <<'FAKE_RELEASE'
#!/bin/sh
set -eu
printf 'RELEASE target=%s\n' "$SOIL_RELEASE_TARGET" >>"$SOIL_TEST_LOG"
[ "${SOIL_FAKE_RELEASE_FAIL-0}" = "0" ] || exit 75
FAKE_RELEASE
  chmod 0755 "$TEST_ROOT/bin/ssh" "$TEST_ROOT/bin/scp" "$TEST_ROOT/fake-release"
  export PATH="$TEST_ROOT/bin:/usr/bin:/bin"
}

test_dry_run() {
  make_local_fixture
  local output
  output=$(SOIL_BOOTSTRAP_DRY_RUN=1 "$BOOTSTRAP_SCRIPT")
  assert_contains "$output" "Bootstrap plan"
  assert_contains "$output" "provisioning, reboot, and release were skipped"
  [[ ! -f "$SOIL_TEST_LOG" ]] || fail "dry run invoked a remote command"
}

test_no_reboot_path() {
  make_local_fixture
  SOIL_FAKE_SPI_READY_INITIAL=1 "$BOOTSTRAP_SCRIPT" >/dev/null
  assert_file_contains "$SOIL_TEST_LOG" "sudo sh /tmp/test-soil-bootstrap/bootstrap-pi.sh /tmp/test-soil-bootstrap alice true"
  assert_file_contains "$SOIL_TEST_LOG" "RELEASE target=alice@pi.example"
  if grep -F "sudo systemctl reboot" "$SOIL_TEST_LOG" >/dev/null; then
    fail "bootstrap rebooted when SPI was already active"
  fi
  [[ "$(grep -c '^SCP ' "$SOIL_TEST_LOG")" -eq 5 ]] || fail "expected five staged assets"
}

test_generic_debian_path_omits_hardware() {
  make_local_fixture
  local output
  output=$("$BOOTSTRAP_SCRIPT" --pi-hat=false)
  assert_contains "$output" "Pi HAT enabled:       false"
  assert_contains "$output" "Hardware setup:       omitted"
  assert_file_contains "$SOIL_TEST_LOG" "bootstrap-pi.sh /tmp/test-soil-bootstrap alice false"
  assert_file_contains "$SOIL_TEST_LOG" "RELEASE target=alice@pi.example"
  if grep -E "spidev|systemctl reboot" "$SOIL_TEST_LOG" >/dev/null; then
    fail "generic bootstrap performed a hardware check or reboot"
  fi
}

test_invalid_hardware_mode() {
  make_local_fixture
  local output
  if output=$("$BOOTSTRAP_SCRIPT" --pi-hat=perhaps 2>&1); then
    fail "invalid hardware mode was accepted"
  fi
  assert_contains "$output" "unknown bootstrap argument"
  [[ ! -f "$SOIL_TEST_LOG" ]] || fail "invalid mode invoked a remote command"
}

test_reboot_path() {
  make_local_fixture
  "$BOOTSTRAP_SCRIPT" >/dev/null
  assert_file_contains "$SOIL_TEST_LOG" "sudo systemctl reboot"
  assert_file_contains "$SOIL_TEST_LOG" "RELEASE target=alice@pi.example"
  [[ -f "$SOIL_TEST_STATE.disconnected" ]] || fail "bootstrap did not observe SSH disconnect"
}

test_bootstrap_failure_stops_release() {
  make_local_fixture
  local output
  if output=$(SOIL_FAKE_BOOTSTRAP_FAIL=1 "$BOOTSTRAP_SCRIPT" 2>&1); then
    fail "remote bootstrap failure returned success"
  fi
  assert_contains "$output" "remote bootstrap failed"
  if grep -F "RELEASE" "$SOIL_TEST_LOG" >/dev/null; then
    fail "release ran after bootstrap failure"
  fi
}

test_invalid_target() {
  make_local_fixture
  local output
  if output=$(SOIL_RELEASE_TARGET='bad target;true' "$BOOTSTRAP_SCRIPT" 2>&1); then
    fail "invalid target was accepted"
  fi
  assert_contains "$output" "must use user@host syntax"
  [[ ! -f "$SOIL_TEST_LOG" ]] || fail "invalid target invoked a remote command"
}

make_pi_fixture() {
  PI_ROOT=$(mktemp -d)
  export PI_ROOT
  trap 'rm -rf -- "$PI_ROOT"' EXIT
  mkdir -p "$PI_ROOT/bin" "$PI_ROOT/root/etc" "$PI_ROOT/root/dev" "$PI_ROOT/stage"
  printf 'ID=debian\nID_LIKE=debian\n' >"$PI_ROOT/root/etc/os-release"
  cp "$SCRIPT_DIR/../deploy/soil-monitor.env.example" "$PI_ROOT/stage/soil-monitor.env.example"
  cp "$SCRIPT_DIR/../deploy/soil-monitor-mosquitto.conf" "$PI_ROOT/stage/soil-monitor-mosquitto.conf"
  cp "$SCRIPT_DIR/../deploy/soil-monitor.service" "$PI_ROOT/stage/soil-monitor.service"
  cp "$SCRIPT_DIR/../deploy/soil-monitor-release.sudoers" "$PI_ROOT/stage/soil-monitor-release.sudoers"
  export SOIL_BOOTSTRAP_ROOT="$PI_ROOT/root"
  export SOIL_TEST_PI_LOG="$PI_ROOT/pi.log"
  export SOIL_TEST_PI_STATE="$PI_ROOT/state"
  export SOIL_FAKE_UID=0
  export SOIL_FAKE_ARCH=aarch64

  cat >"$PI_ROOT/bin/id" <<'FAKE_ID'
#!/bin/sh
set -eu
if [ "${1-}" = "-u" ]; then printf '%s\n' "${SOIL_FAKE_UID-0}"; exit 0; fi
if [ "${1-}" = "alice" ]; then exit 0; fi
if [ "${1-}" = "soilmonitor" ] && [ -f "$SOIL_TEST_PI_STATE.user" ]; then exit 0; fi
exit 1
FAKE_ID

cat >"$PI_ROOT/bin/uname" <<'FAKE_UNAME'
#!/bin/sh
printf '%s\n' "${SOIL_FAKE_ARCH-aarch64}"
FAKE_UNAME

  cat >"$PI_ROOT/bin/apt-get" <<'FAKE_APT'
#!/bin/sh
printf 'APT %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_APT

  cat >"$PI_ROOT/bin/getent" <<'FAKE_GETENT'
#!/bin/sh
set -eu
[ "${1-}" = "group" ] || exit 1
[ -f "$SOIL_TEST_PI_STATE.group-$2" ]
FAKE_GETENT

  cat >"$PI_ROOT/bin/groupadd" <<'FAKE_GROUPADD'
#!/bin/sh
set -eu
group_name=${2-}
printf 'GROUPADD %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
: >"$SOIL_TEST_PI_STATE.group-$group_name"
FAKE_GROUPADD

  cat >"$PI_ROOT/bin/useradd" <<'FAKE_USERADD'
#!/bin/sh
printf 'USERADD %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
: >"$SOIL_TEST_PI_STATE.user"
FAKE_USERADD

  cat >"$PI_ROOT/bin/usermod" <<'FAKE_USERMOD'
#!/bin/sh
printf 'USERMOD %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_USERMOD

  cat >"$PI_ROOT/bin/chown" <<'FAKE_CHOWN'
#!/bin/sh
printf 'CHOWN %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_CHOWN

  cat >"$PI_ROOT/bin/runuser" <<'FAKE_RUNUSER'
#!/bin/sh
set -eu
printf 'RUNUSER %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
[ "${1-}" = "-u" ] && shift 2
[ "${1-}" = "--" ] && shift
exec "$@"
FAKE_RUNUSER

  cat >"$PI_ROOT/bin/sudo" <<'FAKE_SUDO'
#!/bin/sh
printf 'SUDO %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_SUDO

  cat >"$PI_ROOT/bin/install" <<'FAKE_INSTALL'
#!/bin/sh
set -eu
directory=0
mode=0755
while [ "$#" -gt 0 ]; do
  case "$1" in
    -d) directory=1; shift ;;
    -o|-g) shift 2 ;;
    -m) mode=$2; shift 2 ;;
    --) shift; break ;;
    -*) shift ;;
    *) break ;;
  esac
done
if [ "$directory" -eq 1 ]; then
  mkdir -p "$1"
  chmod "$mode" "$1"
else
  [ "$#" -eq 2 ] || exit 64
  mkdir -p "$(dirname "$2")"
  rm -f -- "$2"
  cp "$1" "$2"
  chmod "$mode" "$2"
fi
FAKE_INSTALL

  cat >"$PI_ROOT/bin/raspi-config" <<'FAKE_RASPI'
#!/bin/sh
printf 'RASPI_CONFIG %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_RASPI

  cat >"$PI_ROOT/bin/systemctl" <<'FAKE_SYSTEMCTL'
#!/bin/sh
printf 'SYSTEMCTL %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_SYSTEMCTL

  cat >"$PI_ROOT/bin/visudo" <<'FAKE_VISUDO'
#!/bin/sh
printf 'VISUDO %s\n' "$*" >>"$SOIL_TEST_PI_LOG"
FAKE_VISUDO
  chmod 0755 "$PI_ROOT/bin/"*
  export PATH="$PI_ROOT/bin:/usr/bin:/bin"
}

test_pi_bootstrap_fresh_and_idempotent() {
  make_pi_fixture
  sh "$PI_SCRIPT" "$PI_ROOT/stage" alice true >/dev/null
  assert_file_contains "$PI_ROOT/root/etc/soil-monitor/soil-monitor.env" "SOIL_MQTT_BROKER=tcp://127.0.0.1:1883"
  assert_file_contains "$PI_ROOT/root/etc/mosquitto/conf.d/soil-monitor.conf" "listener 1883"
  assert_file_contains "$PI_ROOT/root/etc/mosquitto/conf.d/soil-monitor.conf" "allow_anonymous true"
  assert_file_contains "$PI_ROOT/root/etc/sudoers.d/soil-monitor-release" "alice ALL=(root)"
  assert_file_contains "$PI_ROOT/root/etc/sudoers.d/soil-monitor-release" "alice ALL=(soilmonitor)"
  assert_file_contains "$SOIL_TEST_PI_LOG" "APT install -y mosquitto mosquitto-clients raspi-config"
  assert_file_contains "$SOIL_TEST_PI_LOG" "RASPI_CONFIG nonint do_spi 0"
  assert_file_contains "$SOIL_TEST_PI_LOG" "SYSTEMCTL enable soil-monitor.service"
  assert_file_contains "$SOIL_TEST_PI_LOG" "SYSTEMCTL restart mosquitto.service"
  assert_file_contains "$SOIL_TEST_PI_LOG" "CHOWN root:soilmonitor $PI_ROOT/root/etc/soil-monitor/soil-monitor.env"
  assert_file_contains "$SOIL_TEST_PI_LOG" "RUNUSER -u soilmonitor -- test -r $PI_ROOT/root/etc/soil-monitor/soil-monitor.env"
  assert_file_contains "$SOIL_TEST_PI_LOG" "RUNUSER -u alice -- sudo -n true"

  local before
  before=$(shasum "$PI_ROOT/root/etc/soil-monitor/soil-monitor.env")
	mkdir -p "$PI_ROOT/root/var/lib/soil-monitor"
	printf 'sqlite-history-preserve-me\n' >"$PI_ROOT/root/var/lib/soil-monitor/soil-monitor.db"
	local database_before database_metadata_before
	database_before=$(shasum "$PI_ROOT/root/var/lib/soil-monitor/soil-monitor.db")
	database_metadata_before=$(ls -ln "$PI_ROOT/root/var/lib/soil-monitor/soil-monitor.db")
  sh "$PI_SCRIPT" "$PI_ROOT/stage" alice true >/dev/null
  [[ "$(shasum "$PI_ROOT/root/etc/soil-monitor/soil-monitor.env")" == "$before" ]] || fail "rerun changed environment"
	[[ "$(shasum "$PI_ROOT/root/var/lib/soil-monitor/soil-monitor.db")" == "$database_before" ]] || fail "rerun changed database contents"
	[[ "$(ls -ln "$PI_ROOT/root/var/lib/soil-monitor/soil-monitor.db")" == "$database_metadata_before" ]] || fail "rerun changed database metadata"
  [[ "$(grep -c '^USERADD ' "$SOIL_TEST_PI_LOG")" -eq 1 ]] || fail "rerun recreated service user"
}

test_pi_bootstrap_preserves_environment() {
  make_pi_fixture
  mkdir -p "$PI_ROOT/root/etc/soil-monitor"
  printf 'SOIL_SECRET=preserve-me\n' >"$PI_ROOT/root/etc/soil-monitor/soil-monitor.env"
  sh "$PI_SCRIPT" "$PI_ROOT/stage" alice true >/dev/null
  [[ "$(cat "$PI_ROOT/root/etc/soil-monitor/soil-monitor.env")" == "SOIL_SECRET=preserve-me" ]] || fail "existing environment was overwritten"
}

test_pi_bootstrap_requires_root() {
  make_pi_fixture
  local output
  if output=$(SOIL_FAKE_UID=1000 sh "$PI_SCRIPT" "$PI_ROOT/stage" alice true 2>&1); then
    fail "non-root bootstrap returned success"
  fi
  assert_contains "$output" "run this script as root"
  [[ ! -f "$SOIL_TEST_PI_LOG" ]] || fail "non-root bootstrap invoked provisioning"
}

test_generic_debian_bootstrap_omits_pi_actions() {
  make_pi_fixture
  SOIL_FAKE_ARCH=x86_64 sh "$PI_SCRIPT" "$PI_ROOT/stage" alice false >/dev/null
  assert_file_contains "$PI_ROOT/root/etc/soil-monitor/soil-monitor.env" "SOIL_PI_HAT_ENABLED=false"
  assert_file_contains "$SOIL_TEST_PI_LOG" "APT install -y mosquitto mosquitto-clients"
  for unexpected in raspi-config RASPI_CONFIG GROUPADD USERMOD; do
    if grep -F "$unexpected" "$SOIL_TEST_PI_LOG" >/dev/null; then
      fail "generic Debian bootstrap invoked $unexpected"
    fi
  done
}

test_generic_bootstrap_preserves_environment() {
  make_pi_fixture
  mkdir -p "$PI_ROOT/root/etc/soil-monitor"
  printf 'SOIL_SECRET=preserve-me\n' >"$PI_ROOT/root/etc/soil-monitor/soil-monitor.env"
  SOIL_FAKE_ARCH=x86_64 sh "$PI_SCRIPT" "$PI_ROOT/stage" alice false >/dev/null
  [[ "$(cat "$PI_ROOT/root/etc/soil-monitor/soil-monitor.env")" == "SOIL_SECRET=preserve-me" ]] || fail "generic bootstrap overwrote existing environment"
}

test_bootstrap_rejects_unsupported_architecture() {
  make_pi_fixture
  local output
  if output=$(SOIL_FAKE_ARCH=riscv64 sh "$PI_SCRIPT" "$PI_ROOT/stage" alice false 2>&1); then
    fail "unsupported architecture was accepted"
  fi
  assert_contains "$output" "unsupported host architecture"
  [[ ! -f "$SOIL_TEST_PI_LOG" ]] || fail "unsupported architecture invoked provisioning"
}

run_test "bootstrap dry-run isolation" test_dry_run
run_test "bootstrap path without reboot" test_no_reboot_path
run_test "bootstrap path with reboot" test_reboot_path
run_test "generic Debian path omits hardware" test_generic_debian_path_omits_hardware
run_test "bootstrap rejects invalid hardware mode" test_invalid_hardware_mode
run_test "bootstrap failure stops release" test_bootstrap_failure_stops_release
run_test "bootstrap rejects invalid target" test_invalid_target
run_test "Pi bootstrap fresh install and idempotency" test_pi_bootstrap_fresh_and_idempotent
run_test "Pi bootstrap preserves environment" test_pi_bootstrap_preserves_environment
run_test "Pi bootstrap requires root" test_pi_bootstrap_requires_root
run_test "generic Debian bootstrap omits Pi actions" test_generic_debian_bootstrap_omits_pi_actions
run_test "generic Debian bootstrap preserves environment" test_generic_bootstrap_preserves_environment
run_test "bootstrap rejects unsupported architecture" test_bootstrap_rejects_unsupported_architecture

printf '%d passed; %d failed\n' "$pass_count" "$fail_count"
[[ "$fail_count" -eq 0 ]]
