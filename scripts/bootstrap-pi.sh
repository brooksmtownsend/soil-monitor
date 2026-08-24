#!/bin/sh
set -eu

die() {
  printf 'bootstrap error: %s\n' "$*" >&2
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

validate_release_user() {
  case "$1" in
    ''|*[!A-Za-z0-9_-]*|[0-9-]*) die "release user must be a valid local account name" ;;
  esac
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' was not found"
}

[ "$#" -eq 3 ] || die "usage: bootstrap-pi.sh <staging-directory> <release-user> <pi-hat-enabled>"
stage=$1
release_user=$2
pi_hat_enabled=$3
validate_absolute_path SOIL_BOOTSTRAP_STAGE_DIR "$stage"
validate_release_user "$release_user"
case "$pi_hat_enabled" in true|false) ;; *) die "pi-hat-enabled must be true or false" ;; esac

[ "$(id -u)" -eq 0 ] || die "run this script as root, for example: sudo sh $stage/bootstrap-pi.sh $stage $release_user"
id "$release_user" >/dev/null 2>&1 || die "release user $release_user does not exist on this host"

root=${SOIL_BOOTSTRAP_ROOT-}
if [ -n "$root" ]; then
  validate_absolute_path SOIL_BOOTSTRAP_ROOT "$root"
fi

os_release="$root/etc/os-release"
[ -f "$os_release" ] || die "cannot identify the operating system from $os_release"
# shellcheck disable=SC1090
. "$os_release"
case "${ID-}:${ID_LIKE-}" in
  debian:*|raspbian:*|*:debian*) ;;
  *) die "this bootstrap supports Debian-family hosts only" ;;
esac

architecture=$(uname -m)
case "$architecture" in
  aarch64|arm64) ;;
  x86_64|amd64)
    [ "$pi_hat_enabled" = "false" ] || die "Pi HAT mode requires a 64-bit ARM Raspberry Pi host"
    ;;
  *) die "unsupported host architecture: $architecture (supported: ARM64 and AMD64)" ;;
esac

for command_name in apt-get chmod chown id install mktemp rm runuser sed sudo systemctl uname useradd visudo; do
  require_command "$command_name"
done
if [ "$pi_hat_enabled" = "true" ]; then
  for command_name in getent groupadd raspi-config usermod; do
    require_command "$command_name"
  done
fi
systemctl --version >/dev/null 2>&1 || die "this bootstrap requires systemd"

environment_source="$stage/soil-monitor.env.example"
mosquitto_source="$stage/soil-monitor-mosquitto.conf"
service_source="$stage/soil-monitor.service"
sudoers_source="$stage/soil-monitor-release.sudoers"
for required_file in "$environment_source" "$mosquitto_source" "$service_source" "$sudoers_source"; do
  [ -f "$required_file" ] || die "staged asset is missing: $required_file"
done

printf 'Installing Debian host packages...\n'
apt-get update
if [ "$pi_hat_enabled" = "true" ]; then
  DEBIAN_FRONTEND=noninteractive apt-get install -y mosquitto mosquitto-clients raspi-config
else
  DEBIAN_FRONTEND=noninteractive apt-get install -y mosquitto mosquitto-clients
fi

if [ "$pi_hat_enabled" = "true" ]; then
  for group_name in gpio spi; do
    if ! getent group "$group_name" >/dev/null 2>&1; then
      groupadd --system "$group_name"
    fi
  done
fi

if ! id soilmonitor >/dev/null 2>&1; then
  useradd --system --home /var/lib/soil-monitor --create-home --shell /usr/sbin/nologin soilmonitor
fi
if [ "$pi_hat_enabled" = "true" ]; then
  usermod -a -G gpio,spi soilmonitor
fi

install -d -o root -g soilmonitor -m 0750 "$root/etc/soil-monitor"
install -d -o soilmonitor -g soilmonitor -m 0750 "$root/var/lib/soil-monitor"
install -d -o root -g root -m 0755 "$root/etc/mosquitto/conf.d"
install -d -o root -g root -m 0755 "$root/etc/systemd/system"
install -d -o root -g root -m 0755 "$root/etc/sudoers.d"

environment="$root/etc/soil-monitor/soil-monitor.env"
if [ -f "$environment" ]; then
  printf 'Preserving existing environment: %s\n' "$environment"
else
  if [ "$pi_hat_enabled" = "true" ]; then
    install -o root -g soilmonitor -m 0640 "$environment_source" "$environment"
  else
    environment_temp=$(mktemp)
    sed 's/^SOIL_PI_HAT_ENABLED=.*/SOIL_PI_HAT_ENABLED=false/' "$environment_source" >"$environment_temp"
    install -o root -g soilmonitor -m 0640 "$environment_temp" "$environment"
    rm -f -- "$environment_temp"
  fi
  printf 'Created initial environment: %s\n' "$environment"
fi
chown root:soilmonitor "$environment"
chmod 0640 "$environment"
runuser -u soilmonitor -- test -r "$environment" ||
  die "$environment is not readable by the soilmonitor service account"

install -o root -g root -m 0644 "$mosquitto_source" "$root/etc/mosquitto/conf.d/soil-monitor.conf"
install -o root -g root -m 0644 "$service_source" "$root/etc/systemd/system/soil-monitor.service"

sudoers_temp=$(mktemp)
trap 'rm -f -- "$sudoers_temp"' EXIT HUP INT TERM
sed "s/^brooks /$release_user /" "$sudoers_source" >"$sudoers_temp"
visudo -cf "$sudoers_temp" >/dev/null
install -o root -g root -m 0440 "$sudoers_temp" "$root/etc/sudoers.d/soil-monitor-release"
runuser -u "$release_user" -- sudo -n true >/dev/null 2>&1 ||
  die "installed release sudo policy is not usable by $release_user"

if [ "$pi_hat_enabled" = "true" ]; then
  printf 'Enabling Raspberry Pi SPI...\n'
  raspi-config nonint do_spi 0
else
  printf 'Pi HAT support disabled; skipping Raspberry Pi hardware provisioning.\n'
fi

systemctl daemon-reload
systemctl enable soil-monitor.service
systemctl enable mosquitto.service
systemctl restart mosquitto.service
systemctl is-active --quiet mosquitto.service || die "mosquitto.service did not become active"

if [ "$pi_hat_enabled" = "false" ]; then
  printf 'Bootstrap complete for generic Debian mode; no hardware reboot is required.\n'
elif [ -e "$root/dev/spidev0.0" ]; then
  printf 'Bootstrap complete; SPI is active and no reboot is required.\n'
else
  printf 'Bootstrap complete; reboot is required before SPI becomes active.\n'
fi
