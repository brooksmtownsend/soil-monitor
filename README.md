# Wireless Soil Monitor

A small Go application that subscribes to MQTT soil-moisture readings for three plants. It can run as a complete Raspberry Pi 3A+ appliance with the Waveshare 1.3-inch SH1106 OLED HAT, or headlessly on a generic Debian server/LXC with the local web dashboard.

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
SOIL_PLANTS="money-tree:Money Tree,pothos:Pothos,snake-plant:Snake Plant"
SOIL_PI_HAT_ENABLED=true
SOIL_SPI_PORT=/dev/spidev0.0
SOIL_BUTTON_DEBOUNCE_MS=150
SOIL_HTTP_ENABLED=true
SOIL_HTTP_ADDR=:8080
```

Set `SOIL_PI_HAT_ENABLED=false` for a host without the OLED HAT. Set `SOIL_HTTP_ENABLED=false` to omit the dashboard listener. Hardware settings are validated only when the HAT is enabled, and the HTTP address is validated only when HTTP is enabled.

The binary can also run directly without any configuration file:

```bash
SOIL_MQTT_BROKER=tcp://127.0.0.1:1883 \
SOIL_PLANTS="money-tree:Money Tree,pothos:Pothos,snake-plant:Snake Plant" \
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

Open the Pi's port 8080 from another device on the same LAN:

```text
http://192.168.1.95:8080/
```

The read-only page shows all three plants, their latest percentages or `--` before a reading arrives, and the MQTT connection state. It refreshes automatically without JavaScript.

Check it from a terminal with:

```bash
curl -fsS http://192.168.1.95:8080/
```

## Publish a test reading

The application subscribes to `plants/+/moisture`. Payloads are integer percentages from `0` through `100`:

```bash
mosquitto_pub -h 192.168.1.95 -t plants/money-tree/moisture -m 47
mosquitto_pub -h 192.168.1.95 -t plants/pothos/moisture -m 72
mosquitto_pub -h 192.168.1.95 -t plants/snake-plant/moisture -m 31
```

Use `-r` to retain a reading across application restarts:

```bash
mosquitto_pub -h 192.168.1.95 -r -t plants/money-tree/moisture -m 47
```

KEY1 selects the previous plant, KEY2 selects the next plant, and KEY3 is currently reserved.

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
