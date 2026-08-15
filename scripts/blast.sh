#!/usr/bin/env bash
set -euo pipefail

readonly INTERVAL="${1:-${SOIL_BLAST_INTERVAL_SECONDS:-0.1}}"
readonly BROKER_HOST="${2:-${SOIL_BLAST_BROKER_HOST:-192.168.1.95}}"
readonly BROKER_PORT="${SOIL_BLAST_BROKER_PORT:-1883}"
readonly TOPIC="plants/test-plant/moisture"

usage() {
  cat <<'EOF'
Usage: ./blast.sh [interval-seconds] [broker-host]

Publish the values 1 through 100 to plants/test-plant/moisture over one MQTT
connection. The application must include test-plant in SOIL_PLANTS.

Environment overrides:
  SOIL_BLAST_INTERVAL_SECONDS  Delay between values (default: 0.1)
  SOIL_BLAST_BROKER_HOST       MQTT broker host (default: 192.168.1.95)
  SOIL_BLAST_BROKER_PORT       MQTT broker port (default: 1883)

Examples:
  ./blast.sh
  ./blast.sh 0.02
  ./blast.sh 0.5 rpi3.local
EOF
}

if [[ "$INTERVAL" == "-h" || "$INTERVAL" == "--help" ]]; then
  usage
  exit 0
fi

[[ "$INTERVAL" =~ ^[0-9]+([.][0-9]+)?$ ]] || {
  printf 'blast error: interval must be a non-negative number of seconds\n' >&2
  exit 2
}
[[ "$BROKER_HOST" =~ ^[A-Za-z0-9._:-]+$ ]] || {
  printf 'blast error: broker host contains unsupported characters\n' >&2
  exit 2
}
if ! [[ "$BROKER_PORT" =~ ^[0-9]+$ ]] || ((BROKER_PORT < 1 || BROKER_PORT > 65535)); then
  printf 'blast error: broker port must be between 1 and 65535\n' >&2
  exit 2
fi
command -v mosquitto_pub >/dev/null 2>&1 || {
  printf 'blast error: mosquitto_pub is required\n' >&2
  exit 1
}

printf 'Publishing 1-100 to %s at %s:%s every %ss\n' \
  "$TOPIC" "$BROKER_HOST" "$BROKER_PORT" "$INTERVAL" >&2
printf 'Ensure test-plant is present in the Pi\047s SOIL_PLANTS configuration.\n' >&2

for value in {1..100}; do
  printf '%s\n' "$value"
  printf 'sent %3d / 100\r' "$value" >&2
  if ((value < 100)); then
    sleep "$INTERVAL"
  fi
done | mosquitto_pub \
  -h "$BROKER_HOST" \
  -p "$BROKER_PORT" \
  -t "$TOPIC" \
  -q 1 \
  -l

printf '\nDone.\n' >&2
