#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "${BASH_SOURCE[0]%/*}" && pwd)"
readonly SCRIPT_DIR
readonly RELEASE_SCRIPT="$SCRIPT_DIR/release.sh"
readonly REMOTE_SCRIPT="$SCRIPT_DIR/release-remote.sh"

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
  if ("$@"); then
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
  unset SOIL_RELEASE_TARGET SOIL_RELEASE_REMOTE_BINARY SOIL_RELEASE_REMOTE_UNIT
  unset SOIL_RELEASE_REMOTE_ENV SOIL_RELEASE_STAGE_DIR SOIL_RELEASE_SERVICE
  unset SOIL_RELEASE_SERVICE_USER
  unset SOIL_RELEASE_VERIFY_ATTEMPTS SOIL_RELEASE_VERIFY_DELAY_SECONDS
  unset SOIL_RELEASE_BINARY_ARM64 SOIL_RELEASE_BINARY_AMD64 SOIL_FAKE_ARCH
  export SOIL_RELEASE_DRY_RUN=0
  mkdir -p "$TEST_ROOT/bin" "$TEST_ROOT/artifacts"
  printf 'binary\n' >"$TEST_ROOT/artifacts/soil-monitor"
  chmod 0755 "$TEST_ROOT/artifacts/soil-monitor"
  printf '[Unit]\nDescription=test\n' >"$TEST_ROOT/artifacts/soil-monitor.service"
  export SOIL_RELEASE_BINARY="$TEST_ROOT/artifacts/soil-monitor"
  export SOIL_RELEASE_UNIT="$TEST_ROOT/artifacts/soil-monitor.service"
  export SOIL_TEST_LOG="$TEST_ROOT/commands.log"
  export SOIL_TEST_SSH_COUNT="$TEST_ROOT/ssh.count"
  export SOIL_TEST_SCP_COUNT="$TEST_ROOT/scp.count"

  cat >"$TEST_ROOT/bin/ssh" <<'FAKE_SSH'
#!/bin/sh
set -eu
count=0
if [ -f "$SOIL_TEST_SSH_COUNT" ]; then count=$(cat "$SOIL_TEST_SSH_COUNT"); fi
count=$((count + 1))
printf '%s\n' "$count" >"$SOIL_TEST_SSH_COUNT"
printf 'SSH %s\n' "$*" >>"$SOIL_TEST_LOG"
case " $* " in
  *' sh -s -- '*) cat >/dev/null ;;
esac
if [ "${SOIL_FAKE_SSH_FAIL_CALL-0}" = "$count" ]; then exit 71; fi
case " $* " in
  *' uname -m '*) printf '%s\n' "${SOIL_FAKE_ARCH-x86_64}" ;;
esac
FAKE_SSH

  cat >"$TEST_ROOT/bin/scp" <<'FAKE_SCP'
#!/bin/sh
set -eu
count=0
if [ -f "$SOIL_TEST_SCP_COUNT" ]; then count=$(cat "$SOIL_TEST_SCP_COUNT"); fi
count=$((count + 1))
printf '%s\n' "$count" >"$SOIL_TEST_SCP_COUNT"
printf 'SCP %s\n' "$*" >>"$SOIL_TEST_LOG"
if [ "${SOIL_FAKE_SCP_FAIL_CALL-0}" = "$count" ]; then exit 72; fi
FAKE_SCP
  chmod 0755 "$TEST_ROOT/bin/ssh" "$TEST_ROOT/bin/scp"
  export PATH="$TEST_ROOT/bin:/usr/bin:/bin"
}

test_default_target_and_dry_run_isolation() {
  make_local_fixture
  local output
  output=$(SOIL_RELEASE_DRY_RUN=1 "$RELEASE_SCRIPT")
  assert_contains "$output" "Target:             brooks@192.168.1.95"
  assert_contains "$output" "Target architecture: detected remotely"
  assert_contains "$output" "SSH and artifact transfer were skipped"
  [[ ! -e "$SOIL_TEST_LOG" ]] || fail "dry run invoked a remote command"
}

test_overridden_target() {
  make_local_fixture
  local output
  output=$(SOIL_RELEASE_DRY_RUN=1 SOIL_RELEASE_TARGET=deploy@example.local "$RELEASE_SCRIPT")
  assert_contains "$output" "Target:             deploy@example.local"
}

test_invalid_overrides() {
  make_local_fixture
  local output
  if output=$(SOIL_RELEASE_DRY_RUN=1 SOIL_RELEASE_TARGET='bad target;true' "$RELEASE_SCRIPT" 2>&1); then
    fail "invalid target was accepted"
  fi
  assert_contains "$output" "must use user@host syntax"
  if output=$(SOIL_RELEASE_DRY_RUN=1 SOIL_RELEASE_REMOTE_BINARY=relative/path "$RELEASE_SCRIPT" 2>&1); then
    fail "relative remote path was accepted"
  fi
  assert_contains "$output" "must be a non-empty absolute path"
}

test_missing_prerequisite() {
  make_local_fixture
  local output
  if output=$(PATH="$TEST_ROOT/empty" /bin/bash "$RELEASE_SCRIPT" 2>&1); then
    fail "missing SSH prerequisite was accepted"
  fi
  assert_contains "$output" "required local command 'ssh' was not found"
}

test_missing_artifact_stops_before_transfer() {
  make_local_fixture
  local output
  if output=$(SOIL_RELEASE_BINARY="$TEST_ROOT/artifacts/missing" "$RELEASE_SCRIPT" 2>&1); then
    fail "missing binary was accepted"
  fi
  assert_contains "$output" "Linux ARM64 binary is missing"
  [[ ! -e "$SOIL_TEST_LOG" ]] || fail "remote command ran after local artifact failure"
}

test_successful_command_ordering() {
  make_local_fixture
  "$RELEASE_SCRIPT" >/dev/null
  local event_count event_1 event_2 event_3 event_4 event_5 event_6
  event_count=$(wc -l <"$SOIL_TEST_LOG")
  event_1=$(sed -n '1p' "$SOIL_TEST_LOG")
  event_2=$(sed -n '2p' "$SOIL_TEST_LOG")
  event_3=$(sed -n '3p' "$SOIL_TEST_LOG")
  event_4=$(sed -n '4p' "$SOIL_TEST_LOG")
  event_5=$(sed -n '5p' "$SOIL_TEST_LOG")
  event_6=$(sed -n '6p' "$SOIL_TEST_LOG")
  [[ "$event_count" -eq 6 ]] || fail "expected six remote events, got $event_count"
  [[ "$event_1" == SSH*true ]] || fail "SSH connectivity was not first"
  [[ "$event_2" == *" uname -m" ]] || fail "architecture detection was not second"
  [[ "$event_3" == *" preflight "* ]] || fail "remote preflight was not third"
  [[ "$event_4" == SCP*soil-monitor ]] || fail "binary transfer was not fourth"
  [[ "$event_5" == SCP*soil-monitor.service ]] || fail "unit transfer was not fifth"
  [[ "$event_6" == *" install "* ]] || fail "remote installation was not last"
}

test_architecture_selects_matching_binary() {
  make_local_fixture
  unset SOIL_RELEASE_BINARY
  printf 'arm64\n' >"$TEST_ROOT/artifacts/soil-monitor-linux-arm64"
  printf 'amd64\n' >"$TEST_ROOT/artifacts/soil-monitor-linux-amd64"
  chmod 0755 "$TEST_ROOT/artifacts/soil-monitor-linux-arm64" "$TEST_ROOT/artifacts/soil-monitor-linux-amd64"
  export SOIL_RELEASE_BINARY_ARM64="$TEST_ROOT/artifacts/soil-monitor-linux-arm64"
  export SOIL_RELEASE_BINARY_AMD64="$TEST_ROOT/artifacts/soil-monitor-linux-amd64"
  SOIL_FAKE_ARCH=x86_64 "$RELEASE_SCRIPT" >/dev/null
  assert_file_contains "$SOIL_TEST_LOG" "$TEST_ROOT/artifacts/soil-monitor-linux-amd64"
}

test_unsupported_architecture_stops_before_transfer() {
  make_local_fixture
  local output
  if output=$(SOIL_FAKE_ARCH=riscv64 "$RELEASE_SCRIPT" 2>&1); then
    fail "unsupported architecture returned success"
  fi
  assert_contains "$output" "unsupported remote architecture"
  if grep -F "SCP " "$SOIL_TEST_LOG" >/dev/null; then
    fail "artifact transferred for unsupported architecture"
  fi
}

test_transfer_failure_skips_install() {
  make_local_fixture
  local output
  if output=$(SOIL_FAKE_SCP_FAIL_CALL=2 "$RELEASE_SCRIPT" 2>&1); then
    fail "transfer failure returned success"
  fi
  assert_contains "$output" "remote installation was not attempted"
  if grep -F " install " "$SOIL_TEST_LOG" >/dev/null; then
    fail "installation ran after transfer failure"
  fi
  assert_file_contains "$SOIL_TEST_LOG" " cleanup "
}

make_remote_fixture() {
  REMOTE_ROOT=$(mktemp -d)
  export REMOTE_ROOT
  trap 'rm -rf -- "$REMOTE_ROOT"' EXIT
  mkdir -p "$REMOTE_ROOT/bin" "$REMOTE_ROOT/stage" "$REMOTE_ROOT/dest" "$REMOTE_ROOT/etc"
  printf 'new-binary\n' >"$REMOTE_ROOT/stage/soil-monitor"
  printf 'new-unit\n' >"$REMOTE_ROOT/stage/soil-monitor.service"
  printf 'SOIL_SECRET=preserve-me\n' >"$REMOTE_ROOT/etc/soil-monitor.env"
  export SOIL_TEST_REMOTE_LOG="$REMOTE_ROOT/remote.log"
  export SOIL_TEST_ACTIVE_COUNT="$REMOTE_ROOT/active.count"

  cat >"$REMOTE_ROOT/bin/sudo" <<'FAKE_SUDO'
#!/bin/sh
set -eu
printf 'SUDO %s\n' "$*" >>"$SOIL_TEST_REMOTE_LOG"
if [ "${SOIL_FAKE_ENV_UNREADABLE-0}" = "1" ] && [ "${6-}" = "$REMOTE_ROOT/etc/soil-monitor.env" ]; then
  exit 77
fi
[ "${1-}" = "-n" ] && shift
if [ "${1-}" = "-u" ]; then shift 2; fi
exec "$@"
FAKE_SUDO

  cat >"$REMOTE_ROOT/bin/id" <<'FAKE_ID'
#!/bin/sh
[ "${1-}" = "soilmonitor" ]
FAKE_ID

  cat >"$REMOTE_ROOT/bin/install" <<'FAKE_INSTALL'
#!/bin/sh
set -eu
mode=0755
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o|-g) shift 2 ;;
    -m) mode=$2; shift 2 ;;
    --) shift; break ;;
    -*) shift ;;
    *) break ;;
  esac
done
[ "$#" -eq 2 ] || exit 64
cp "$1" "$2"
chmod "$mode" "$2"
FAKE_INSTALL

  cat >"$REMOTE_ROOT/bin/systemd-analyze" <<'FAKE_ANALYZE'
#!/bin/sh
printf 'ANALYZE %s\n' "$*" >>"$SOIL_TEST_REMOTE_LOG"
exit "${SOIL_FAKE_ANALYZE_EXIT-0}"
FAKE_ANALYZE

  cat >"$REMOTE_ROOT/bin/systemctl" <<'FAKE_SYSTEMCTL'
#!/bin/sh
set -eu
printf 'SYSTEMCTL %s\n' "$*" >>"$SOIL_TEST_REMOTE_LOG"
if [ "${1-}" = "is-active" ]; then
  count=0
  if [ -f "$SOIL_TEST_ACTIVE_COUNT" ]; then count=$(cat "$SOIL_TEST_ACTIVE_COUNT"); fi
  count=$((count + 1))
  printf '%s\n' "$count" >"$SOIL_TEST_ACTIVE_COUNT"
  [ "$count" -ge "${SOIL_FAKE_ACTIVE_AFTER-1}" ]
fi
FAKE_SYSTEMCTL

  cat >"$REMOTE_ROOT/bin/journalctl" <<'FAKE_JOURNAL'
#!/bin/sh
printf 'JOURNAL %s\n' "$*" >>"$SOIL_TEST_REMOTE_LOG"
FAKE_JOURNAL
  chmod 0755 "$REMOTE_ROOT/bin/"*
  export PATH="$REMOTE_ROOT/bin:/usr/bin:/bin"
}

run_remote_install() {
  sh "$REMOTE_SCRIPT" install \
    "$REMOTE_ROOT/stage" \
    "$REMOTE_ROOT/dest/soil-monitor" \
    "$REMOTE_ROOT/dest/soil-monitor.service" \
    "$REMOTE_ROOT/etc/soil-monitor.env" \
    soil-monitor.service soilmonitor 1 0
}

run_remote_preflight() {
  sh "$REMOTE_SCRIPT" preflight \
    "$REMOTE_ROOT/stage" \
    "$REMOTE_ROOT/dest/soil-monitor" \
    "$REMOTE_ROOT/dest/soil-monitor.service" \
    "$REMOTE_ROOT/etc/soil-monitor.env" \
    soil-monitor.service soilmonitor
}

test_remote_preflight_uses_service_account() {
  make_remote_fixture
  run_remote_preflight >/dev/null
  assert_file_contains "$SOIL_TEST_REMOTE_LOG" "SUDO -n -u soilmonitor test -r $REMOTE_ROOT/etc/soil-monitor.env"
}

test_remote_preflight_reports_unreadable_environment() {
  make_remote_fixture
  local output
  if output=$(SOIL_FAKE_ENV_UNREADABLE=1 run_remote_preflight 2>&1); then
    fail "unreadable service environment returned success"
  fi
  assert_contains "$output" "exists but is not readable by soilmonitor"
}

test_remote_preflight_allows_absent_environment() {
  make_remote_fixture
  rm -f "$REMOTE_ROOT/etc/soil-monitor.env"
  local output
  output=$(run_remote_preflight)
  assert_contains "$output" "Optional environment file"
}

test_remote_install_allows_absent_environment() {
  make_remote_fixture
  rm -f "$REMOTE_ROOT/etc/soil-monitor.env"
  SOIL_FAKE_ACTIVE_AFTER=1 run_remote_install >/dev/null
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor")" == "new-binary" ]] || fail "new binary was not installed without environment file"
}

test_remote_success_preserves_environment() {
  make_remote_fixture
  local before
  before=$(shasum "$REMOTE_ROOT/etc/soil-monitor.env")
  SOIL_FAKE_ACTIVE_AFTER=1 run_remote_install >/dev/null
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor")" == "new-binary" ]] || fail "new binary was not installed"
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor.service")" == "new-unit" ]] || fail "new unit was not installed"
  [[ "$(shasum "$REMOTE_ROOT/etc/soil-monitor.env")" == "$before" ]] || fail "environment file changed"
  assert_file_contains "$SOIL_TEST_REMOTE_LOG" "SYSTEMCTL daemon-reload"
  assert_file_contains "$SOIL_TEST_REMOTE_LOG" "SYSTEMCTL restart soil-monitor.service"
}

test_health_failure_diagnostics_and_rollback() {
  make_remote_fixture
  printf 'old-binary\n' >"$REMOTE_ROOT/dest/soil-monitor"
  printf 'old-unit\n' >"$REMOTE_ROOT/dest/soil-monitor.service"
  local before output
  before=$(shasum "$REMOTE_ROOT/etc/soil-monitor.env")
  if output=$(SOIL_FAKE_ACTIVE_AFTER=2 run_remote_install 2>&1); then
    fail "unhealthy release returned success"
  fi
  assert_contains "$output" "collecting diagnostics"
  assert_contains "$output" "Rollback succeeded"
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor")" == "old-binary" ]] || fail "binary was not rolled back"
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor.service")" == "old-unit" ]] || fail "unit was not rolled back"
  [[ "$(shasum "$REMOTE_ROOT/etc/soil-monitor.env")" == "$before" ]] || fail "environment file changed"
  assert_file_contains "$SOIL_TEST_REMOTE_LOG" "SYSTEMCTL status --no-pager --full soil-monitor.service"
  assert_file_contains "$SOIL_TEST_REMOTE_LOG" "JOURNAL -u soil-monitor.service -n 50 --no-pager"
}

test_unit_validation_failure_uses_rollback() {
  make_remote_fixture
  printf 'old-binary\n' >"$REMOTE_ROOT/dest/soil-monitor"
  printf 'old-unit\n' >"$REMOTE_ROOT/dest/soil-monitor.service"
  local output
  if output=$(SOIL_FAKE_ANALYZE_EXIT=1 SOIL_FAKE_ACTIVE_AFTER=1 run_remote_install 2>&1); then
    fail "invalid systemd unit returned success"
  fi
  assert_contains "$output" "Installed systemd unit failed validation"
  assert_contains "$output" "Rollback succeeded"
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor")" == "old-binary" ]] || fail "binary was not rolled back after validation failure"
  [[ "$(cat "$REMOTE_ROOT/dest/soil-monitor.service")" == "old-unit" ]] || fail "unit was not rolled back after validation failure"
}

test_first_deployment_failure_reports_no_backup() {
  make_remote_fixture
  local output
  if output=$(SOIL_FAKE_ACTIVE_AFTER=99 run_remote_install 2>&1); then
    fail "unhealthy first deployment returned success"
  fi
  assert_contains "$output" "Automatic rollback unavailable"
}

run_test "default target and dry-run isolation" test_default_target_and_dry_run_isolation
run_test "overridden target" test_overridden_target
run_test "invalid overrides" test_invalid_overrides
run_test "missing local prerequisite" test_missing_prerequisite
run_test "local artifact failure before transfer" test_missing_artifact_stops_before_transfer
run_test "successful remote command ordering" test_successful_command_ordering
run_test "architecture selects matching binary" test_architecture_selects_matching_binary
run_test "unsupported architecture stops before transfer" test_unsupported_architecture_stops_before_transfer
run_test "transfer failure isolation and cleanup" test_transfer_failure_skips_install
run_test "remote preflight uses service account" test_remote_preflight_uses_service_account
run_test "remote preflight reports unreadable environment" test_remote_preflight_reports_unreadable_environment
run_test "remote preflight allows absent environment" test_remote_preflight_allows_absent_environment
run_test "remote install allows absent environment" test_remote_install_allows_absent_environment
run_test "successful install preserves environment" test_remote_success_preserves_environment
run_test "health failure diagnostics and rollback" test_health_failure_diagnostics_and_rollback
run_test "unit validation failure uses rollback" test_unit_validation_failure_uses_rollback
run_test "first deployment reports unavailable rollback" test_first_deployment_failure_reports_no_backup

printf '%d passed; %d failed\n' "$pass_count" "$fail_count"
[[ "$fail_count" -eq 0 ]]
