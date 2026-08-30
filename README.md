# Wireless Soil Monitor

A small Go application that discovers MQTT soil-moisture sensors, stores their history locally, and displays any number of plants. It can run as a complete Raspberry Pi 3A+ appliance with the Waveshare 1.3-inch SH1106 OLED HAT, or headlessly on a generic Debian server/LXC with the local web dashboard.

The Pi also runs Mosquitto, so the appliance is self-contained: sensor nodes publish readings to the Pi, and the single soil-monitor binary drives both the OLED and dashboard.

## Quick start

Start with a 64-bit Raspberry Pi OS/Debian Lite image that already has Wi-Fi, SSH, and a sudo-capable user configured. The default deployment target is `brooks@192.168.1.95`.

For a freshly imaged Pi:

```bash
mise install
mise run bootstrap
```

Bootstrap installs and configures Mosquitto, enables SPI, creates the service account and environment, reboots if necessary, and performs the first application release.

For a systemd-based AMD64 or ARM64 Debian server/LXC without the Pi HAT:

```bash
SOIL_RELEASE_TARGET=brooks@soil-monitor.local mise run bootstrap -- --pi-hat=false
```

Generic mode installs the same Mosquitto and soil-monitor services but does not install `raspi-config`, create GPIO/SPI groups, enable SPI, inspect hardware devices, or reboot for hardware activation.

For later application releases:

```bash
mise run release
```

Use a different address or hostname with:

```bash
SOIL_RELEASE_TARGET=brooks@rpi3.local mise run bootstrap
```

Bootstrap requires the host to already be reachable over SSH with sudo and systemd. It does not configure or recover networking. Mosquitto and the dashboard are intended for a trusted local network and do not provide authentication or TLS. Releases detect ARM64 or AMD64 remotely and select the matching static binary.

## Configuration

Bootstrap stores service configuration at:

```text
/etc/soil-monitor/soil-monitor.env
```

Bootstrap creates it when missing and preserves its contents on later runs. The default configuration is:

```dotenv
SOIL_MQTT_BROKER=tcp://127.0.0.1:1883
SOIL_MQTT_CLIENT_ID=kitchen-soil-display
SOIL_DATABASE_PATH=/var/lib/soil-monitor/soil-monitor.db
SOIL_PI_HAT_ENABLED=true
SOIL_SPI_PORT=/dev/spidev0.0
SOIL_BUTTON_DEBOUNCE_MS=150
SOIL_HTTP_ENABLED=true
SOIL_HTTP_ADDR=:80
```

Set `SOIL_PI_HAT_ENABLED=false`

The binary can also run directly without any configuration file:

```bash
SOIL_MQTT_BROKER=tcp://127.0.0.1:1883 \
SOIL_DATABASE_PATH=./soil-monitor.db \
./build/soil-monitor --pi-hat=false
```

Alternatively, pass `--config /path/to/soil-monitor.env`. The file accepts blank lines, `#` comments, and unquoted or single/double-quoted `KEY=VALUE` entries from the documented `SOIL_*` settings; it is parsed as data and does not execute shell expansion. Precedence is command-line flags, process environment, the selected file, then built-in defaults. Run `soil-monitor --help` for every flag and environment mapping.

The systemd environment file is optional to the unit and is preserved byte-for-byte by routine releases. Bootstrap creates it when missing. If it is removed, required variables must instead be injected with a systemd drop-in or another service-manager mechanism.

After editing it, restart the application:

```bash
sudoedit /etc/soil-monitor/soil-monitor.env
sudo systemctl restart soil-monitor.service
```

## Dashboard

Open the Pi's port 80 from another device on the same LAN:

```text
http://192.168.1.95/
```

The page shows every discovered plant, its latest raw value and percentage, and the MQTT connection state. Select a plant to view an inline history chart and table, or assign a friendly display name. Plant history defaults to the rolling `1d` range; `1w`, `1mo`, `3mo`, `6mo`, and `1y` controls select longer ranges. Day and week ranges cover the preceding 24 hours or 7 days. Month and year ranges use UTC calendar dates and clamp to the target month's final day when needed. Each result contains at most the newest 1,000 readings in the selected range.

Latest values update over the local WebSocket connection without a full-page reload. History remains a server-rendered snapshot and refreshes when a range is selected or the detail page is reloaded. A range can also be bookmarked directly, such as `/plants/money-tree?range=3mo`.

The dashboard's name form changes persistent application data. Port 80 remains an unauthenticated trusted-LAN service, so it must not be exposed to an untrusted network.

> **Migration note:** The default HTTP port changed from 8080 to 80 in this release. If your deployment relied on the old default, add `SOIL_HTTP_ADDR=:8080` to `/etc/soil-monitor/soil-monitor.env` to preserve the previous behavior.

Check it from a terminal with:

```bash
curl -fsS http://192.168.1.95/
```

## Publish a test reading

The application subscribes to `plants/+/moisture` and discovers kebab-case plant IDs automatically. Payloads are JSON objects with a non-negative integer `raw` reading and an integer `percentage` from `0` through `100`:

```bash
mosquitto_pub -h 192.168.1.95 -t plants/money-tree/moisture -m '{"raw":32768,"percentage":47}'
mosquitto_pub -h 192.168.1.95 -t plants/pothos/moisture -m '{"raw":30100,"percentage":72}'
mosquitto_pub -h 192.168.1.95 -t plants/snake-plant/moisture -m '{"raw":41000,"percentage":31}'
```

Use `-r` to retain a reading across application restarts:

```bash
mosquitto_pub -h 192.168.1.95 -r -t plants/money-tree/moisture -m '{"raw":32768,"percentage":47}'
```

KEY1 selects the previous plant, KEY2 selects the next plant, and KEY3 is currently reserved.

## Reading history and upgrades

SQLite history is stored at `SOIL_DATABASE_PATH`, defaulting to `/var/lib/soil-monitor/soil-monitor.db`. Timestamps record when the monitor receives each reading in UTC. Because retained MQTT payloads contain no stable reading ID, reconnecting or restarting can record the retained value as a new observation.

History is retained without automatic pruning. Monitor the size of `/var/lib/soil-monitor`, and back up the database while the service is stopped or with SQLite's online backup tooling. Bootstrap preserves an existing database and routine releases never transfer or replace it.

The JSON payload is a breaking protocol change. Upgrade every publisher together with the monitor; the old binary requires integer payloads, while the new binary rejects them. To roll back, restore the previous monitor binary and unit, restore publishers to integer payloads, and leave the SQLite database in place for a later upgrade. Legacy `SOIL_PLANTS` values can remain temporarily for rollback but are ignored by the new binary.

## Local development and testing

Developer tools and routine commands are managed by `mise`:

```bash
mise install
mise run
```

Run the automated suite and static analysis without Raspberry Pi hardware:

```bash
mise run test
mise run vet
mise run lint-shell
```

Build for the local machine, ARM64 Linux, or AMD64 Linux:

```bash
mise run build
mise run build-pi
mise run build-linux-arm64
mise run build-linux-amd64
```

Preview deployment without contacting the Pi:

```bash
SOIL_RELEASE_DRY_RUN=1 mise run release
SOIL_BOOTSTRAP_DRY_RUN=1 mise run bootstrap
SOIL_BOOTSTRAP_DRY_RUN=1 mise run bootstrap -- --pi-hat=false
```

Dry runs validate local inputs and artifacts but do not connect to the host. Failed activation automatically restores `.previous` binary and unit backups when both exist. For manual rollback, reinstall `/usr/local/bin/soil-monitor.previous` and `/etc/systemd/system/soil-monitor.service.previous` to their unsuffixed paths, then run `sudo systemctl daemon-reload` and restart `soil-monitor.service`.

## Development workflow

Development in this repository uses OpenSpec and small, focused changes so implementation, tests, and specifications stay synchronized. Contributors and coding agents should inspect `openspec/specs/`, `mise tasks`, and the relevant code and tests for detailed behavior rather than expanding this README with internal documentation.
