# Wireless Soil Monitor MVP

This project builds one Go binary for a Raspberry Pi 3A+ with the Waveshare 1.3-inch OLED HAT. It subscribes to MQTT moisture values for exactly three plants, keeps the latest values in memory, shows one plant at a time, uses the three labeled HAT buttons for navigation, and serves the same current readings as a minimal local web page.

The MVP intentionally has no sensor input, historical storage, calibration, or alerts. Host bootstrap provisions a Pi-local Mosquitto broker for the prototype.

## Hardware target

The implementation targets the Waveshare **1.3inch OLED HAT**, part number `1.3inch OLED HAT` / SKU 13890, in its factory-default four-wire SPI configuration. Waveshare documents an SH1106 controller with a 128x64 display and the following BCM assignments:

| Function | BCM GPIO |
| --- | ---: |
| SPI clock | 11 |
| SPI MOSI | 10 |
| SPI CE0 / chip select | 8 |
| Display data/command | 24 |
| Display reset | 25 |
| KEY1 / previous plant | 21 |
| KEY2 / next plant | 20 |
| KEY3 / reserved | 16 |

The pinout and factory SPI mode come from the [Waveshare product documentation](https://www.waveshare.com/wiki/1.3inch_OLED_HAT). A board modified by resoldering its interface-selection resistors for I2C is not supported by this MVP.

## MQTT contract

The application subscribes to:

```text
plants/+/moisture
```

Each configured plant uses its identifier in the topic and a plain integer percentage from `0` through `100` as the payload:

```text
plants/money-tree/moisture  ->  47
plants/pothos/moisture      ->  72
plants/snake-plant/moisture ->  31
```

Messages with an unknown plant, malformed integer, or value outside `0..100` are logged and ignored. Values remain in memory through MQTT disconnects but are lost when the process restarts. MQTT retained publications can be used when values should return after a restart.

## Configuration

The process reads environment variables at startup:

| Variable | Required | Example | Meaning |
| --- | --- | --- | --- |
| `SOIL_MQTT_BROKER` | Yes | `tcp://127.0.0.1:1883` | MQTT broker URL using `tcp`, `tls`, `ws`, or `wss`; bootstrap installs Mosquitto on the same Pi |
| `SOIL_PLANTS` | Yes | `money-tree:Money Tree,pothos:Pothos,snake-plant:Snake Plant` | Exactly three comma-separated `id:name` entries |
| `SOIL_MQTT_CLIENT_ID` | No | `kitchen-soil-display` | MQTT client ID; defaults to `soil-monitor` |
| `SOIL_SPI_PORT` | No | `/dev/spidev0.0` | SPI CE0 device used by the factory-configured HAT |
| `SOIL_BUTTON_DEBOUNCE_MS` | No | `150` | Positive debounce interval in milliseconds |
| `SOIL_HTTP_ADDR` | No | `:8080` | HTTP listen address for the read-only local web display |

Plant identifiers must be unique kebab-case values. Display names must be non-empty; long names are truncated on screen without changing their configured value.

## Build and test

Developer tooling is pinned through `mise`:

```bash
mise install
mise run
```

Running `mise run` without a task name lists the available project tasks. Run the test suite and static analysis with:

```bash
mise run test
mise run vet
```

Build a local binary or a static ARM64 Linux binary for 64-bit Debian on the Pi:

```bash
mise run build
mise run build-pi
```

The Pi artifact is written to `build/soil-monitor-linux-arm64`.

## Bootstrap a fresh Pi

The bootstrap boundary is a fresh 64-bit Debian/Raspberry Pi OS Lite image that has already joined Wi-Fi and accepts key-based SSH for a sudo-capable account. Raspberry Pi Imager can preconfigure the username, wireless network, hostname, and SSH public key. Bootstrap intentionally does not store or modify Wi-Fi credentials and cannot repair a host that is already unreachable.

Verify the target before beginning:

```bash
ssh -o BatchMode=yes brooks@192.168.1.95 true
ssh -t brooks@192.168.1.95 sudo -v
```

After a reimage, SSH may correctly reject the old saved host key. Compare the new fingerprint with the Pi or image information before removing the stale entry; never disable host-key verification as a shortcut:

```bash
ssh-keygen -R 192.168.1.95
ssh brooks@192.168.1.95
```

From the repository, provision the complete appliance and perform its first release:

```bash
mise install
mise run bootstrap
```

The task validates and cross-builds locally, stages the bootstrap assets, prompts for remote sudo, installs and configures Mosquitto, enables SPI, creates the `soilmonitor` account and environment, installs the release sudo policy, and enables the services. If SPI is not active yet, it reboots once, waits a bounded time for SSH to return, verifies `/dev/spidev0.0`, and invokes the same guarded release used by `mise run release`. Existing `/etc/soil-monitor/soil-monitor.env` content is preserved on reruns.

Preview the plan without connecting to the Pi:

```bash
SOIL_BOOTSTRAP_DRY_RUN=1 mise run bootstrap
```

Bootstrap and routine release use the same target:

```bash
SOIL_RELEASE_TARGET=brooks@rpi3.local mise run bootstrap
```

Wait behavior can be adjusted for an unusually slow Pi or network:

| Variable | Default | Meaning |
| --- | ---: | --- |
| `SOIL_BOOTSTRAP_STAGE_DIR` | `/tmp/soil-monitor-bootstrap` | Unprivileged remote staging directory |
| `SOIL_BOOTSTRAP_SPI_PORT` | `/dev/spidev0.0` | Device that must exist before release |
| `SOIL_BOOTSTRAP_DISCONNECT_ATTEMPTS` | `15` | Checks allowed while waiting for reboot to begin |
| `SOIL_BOOTSTRAP_RECONNECT_ATTEMPTS` | `60` | Checks allowed while waiting for SSH to return |
| `SOIL_BOOTSTRAP_WAIT_SECONDS` | `2` | Delay between checks |

### Run only the Pi bootstrap script

The privileged portion is standalone when an explicit `scp` and SSH workflow is preferable. The four managed assets must be staged beside it:

```bash
ssh brooks@192.168.1.95 'mkdir -p -m 700 /tmp/soil-monitor-bootstrap'
scp \
  scripts/bootstrap-pi.sh \
  deploy/soil-monitor.env.example \
  deploy/soil-monitor-mosquitto.conf \
  deploy/soil-monitor.service \
  deploy/soil-monitor-release.sudoers \
  brooks@192.168.1.95:/tmp/soil-monitor-bootstrap/
ssh -t brooks@192.168.1.95 \
  'sudo sh /tmp/soil-monitor-bootstrap/bootstrap-pi.sh /tmp/soil-monitor-bootstrap brooks'
```

If it reports that SPI requires a reboot, reboot and confirm the device before running the regular release:

```bash
ssh -t brooks@192.168.1.95 sudo systemctl reboot
ssh brooks@192.168.1.95 'test -e /dev/spidev0.0'
mise run release
```

Mosquitto listens on TCP port 1883 and permits anonymous clients so sensor publishers can use it from the trusted home LAN. This prototype configuration has no TLS or authentication and must not be exposed to the internet or an untrusted network.

## Manual host setup

The following is the manual alternative to `mise run bootstrap` and is also useful for inspecting what it provisions.

Install and configure the Pi-local broker:

```bash
scp deploy/soil-monitor-mosquitto.conf brooks@192.168.1.95:/tmp/
ssh -t brooks@192.168.1.95 \
  'sudo apt update &&
   sudo apt install mosquitto mosquitto-clients raspi-config &&
   sudo install -o root -g root -m 0644 /tmp/soil-monitor-mosquitto.conf /etc/mosquitto/conf.d/soil-monitor.conf &&
   sudo systemctl enable --now mosquitto.service'
```

Enable SPI, then reboot:

```bash
sudo raspi-config
```

Choose `Interface Options`, then `SPI`, then enable it. After reboot, confirm that CE0 exists:

```bash
ls -l /dev/spidev0.0
```

Create a service account and installation directories:

```bash
sudo useradd --system --home /var/lib/soil-monitor --create-home --shell /usr/sbin/nologin soilmonitor
sudo usermod -a -G gpio,spi soilmonitor
sudo install -d -o root -g soilmonitor -m 0750 /etc/soil-monitor
```

Copy the cross-compiled binary, service file, and environment example to the Pi, then install them:

```bash
sudo install -o root -g root -m 0755 build/soil-monitor-linux-arm64 /usr/local/bin/soil-monitor
sudo install -o root -g root -m 0644 deploy/soil-monitor.service /etc/systemd/system/soil-monitor.service
sudo install -o root -g soilmonitor -m 0640 deploy/soil-monitor.env.example /etc/soil-monitor/soil-monitor.env
sudoedit /etc/soil-monitor/soil-monitor.env
```

Run the binary manually before enabling the service:

```bash
sudo -u soilmonitor sh -c 'set -a; . /etc/soil-monitor/soil-monitor.env; exec /usr/local/bin/soil-monitor'
```

Press Ctrl-C after confirming that the initial plant and `MQTT OFFLINE` or a connected view appear. The process should exit without requiring a forced kill.

## Bench-test MQTT

Install an MQTT publishing client on another computer and use the Pi's address. Publish all three values:

```bash
mosquitto_pub -h 192.168.1.95 -t plants/money-tree/moisture -m 47
mosquitto_pub -h 192.168.1.95 -t plants/pothos/moisture -m 72
mosquitto_pub -h 192.168.1.95 -t plants/snake-plant/moisture -m 31
```

For retained values that return after application restarts, add `-r`:

```bash
mosquitto_pub -h 192.168.1.95 -r -t plants/money-tree/moisture -m 47
```

Exercise rejection behavior without changing the last valid value:

```bash
mosquitto_pub -h 192.168.1.95 -t plants/money-tree/moisture -m wet
mosquitto_pub -h 192.168.1.95 -t plants/money-tree/moisture -m 101
mosquitto_pub -h 192.168.1.95 -t plants/unknown/moisture -m 50
```

Use KEY1 and KEY2 to move backward and forward through the plants. Navigation must wrap between plant 1 and plant 3, and one physical press must produce one move. KEY3 is reserved and must leave the view unchanged. Stop and restart the broker to verify that `MQTT OFFLINE` appears while the last reading remains visible, then disappears after reconnection.

## Local web display

The same binary serves a small, read-only HTML status page. With the default `SOIL_HTTP_ADDR=:8080`, open this URL from another device on the same LAN:

```text
http://192.168.1.95:8080/
```

The page shows all three configured plant names, each latest in-memory moisture percentage or `--` before its first reading, and the current MQTT online/offline state. It refreshes itself periodically using HTML metadata; there is no JavaScript, API, external asset directory, or separate web service.

Verify the response from a terminal after releasing:

```bash
curl -fsS http://192.168.1.95:8080/
```

To use a different port or restrict the page to the Pi itself, set the host-managed environment file and restart the service:

```bash
SOIL_HTTP_ADDR=:9090
```

```bash
SOIL_HTTP_ADDR=127.0.0.1:8080
```

The default address listens on the Pi's network interfaces. The page has no authentication or TLS and is intended only for a trusted local network; do not expose its port directly to the internet. HTTP requests cannot change readings, plant selection, MQTT topics, or application configuration.

## Run as a service

After the manual checks pass:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now soil-monitor.service
sudo systemctl status soil-monitor.service
sudo journalctl -u soil-monitor.service -f
```

Verify restart and graceful stop behavior:

```bash
sudo systemctl restart soil-monitor.service
sudo systemctl stop soil-monitor.service
sudo systemctl show soil-monitor.service -p Result -p ExecMainStatus
sudo systemctl start soil-monitor.service
```

`Result=success` and `ExecMainStatus=0` after a normal stop indicate that signal-driven cleanup completed successfully.

## Automated releases

After the one-time host setup below, a release validates the project, builds the static Linux ARM64 binary, stages the binary and unit over SSH, installs them with fixed ownership and modes, restarts the service, and verifies that it becomes active:

```bash
mise run release
```

The default target is `brooks@192.168.1.95`. Routine releases preserve `/etc/soil-monitor/soil-monitor.env`; the repository example is only for initial setup.

### One-time release setup

Install the local tools and configure key-based SSH authentication:

```bash
mise install
ssh-keygen -t ed25519
ssh-copy-id brooks@192.168.1.95
ssh -o BatchMode=yes brooks@192.168.1.95 true
```

The Pi must already have the `soilmonitor` service account, its `gpio` and `spi` supplementary groups, SPI enabled, and the environment file created as described in **Manual host setup**. `mise run bootstrap` establishes these prerequisites automatically. Confirm the host-managed environment file before enabling releases:

```bash
ssh brooks@192.168.1.95 'sudo test -f /etc/soil-monitor/soil-monitor.env'
```

Install the repository's narrow sudo policy and validate it before ending the privileged session:

```bash
scp deploy/soil-monitor-release.sudoers brooks@192.168.1.95:/tmp/soil-monitor-release.sudoers
ssh -t brooks@192.168.1.95 \
  'sudo install -o root -g root -m 0440 /tmp/soil-monitor-release.sudoers /etc/sudoers.d/soil-monitor-release && sudo visudo -cf /etc/sudoers.d/soil-monitor-release && rm /tmp/soil-monitor-release.sudoers'
ssh brooks@192.168.1.95 'sudo -n true'
```

The policy permits `brooks` to install only the soil-monitor binary and unit from `/tmp/soil-monitor-release`, manage only `soil-monitor.service`, and read its failure diagnostics. If any default remote path, staging directory, service name, or SSH account is changed, review and update the policy before releasing.

Release preflight checks the protected environment by running an exact read test as the `soilmonitor` service account. This avoids falsely reporting the file as missing when the SSH user correctly cannot traverse `/etc/soil-monitor`. Bootstrap installs the matching narrow sudo rule and normalizes the environment metadata without changing its content:

```text
root:soilmonitor 0640 /etc/soil-monitor/soil-monitor.env
```

### Preview and run a release

An effect-free preview still runs the local test, vet, shell-lint, and ARM64 build tasks, but it does not open SSH connections or transfer files:

```bash
SOIL_RELEASE_DRY_RUN=1 mise run release
```

A successful live release ends with output similar to:

```text
soil-monitor.service is active.
Release successful: soil-monitor.service is active on brooks@192.168.1.95.
```

The following prefixed variables override release defaults:

| Variable | Default |
| --- | --- |
| `SOIL_RELEASE_TARGET` | `brooks@192.168.1.95` |
| `SOIL_RELEASE_BINARY` | `<repository>/build/soil-monitor-linux-arm64` |
| `SOIL_RELEASE_UNIT` | `<repository>/deploy/soil-monitor.service` |
| `SOIL_RELEASE_STAGE_DIR` | `/tmp/soil-monitor-release` |
| `SOIL_RELEASE_REMOTE_BINARY` | `/usr/local/bin/soil-monitor` |
| `SOIL_RELEASE_REMOTE_UNIT` | `/etc/systemd/system/soil-monitor.service` |
| `SOIL_RELEASE_REMOTE_ENV` | `/etc/soil-monitor/soil-monitor.env` |
| `SOIL_RELEASE_SERVICE_USER` | `soilmonitor` |
| `SOIL_RELEASE_SERVICE` | `soil-monitor.service` |
| `SOIL_RELEASE_VERIFY_ATTEMPTS` | `10` |
| `SOIL_RELEASE_VERIFY_DELAY_SECONDS` | `1` |

For example, preview a replacement host without contacting it:

```bash
SOIL_RELEASE_DRY_RUN=1 \
SOIL_RELEASE_TARGET=brooks@soil-display.local \
mise run release
```

### Failure diagnostics and rollback

If the new service does not become active within the bounded verification interval, the release prints `systemctl status` and the latest 50 journal entries. Existing installed artifacts are copied to these paths before replacement:

```text
/usr/local/bin/soil-monitor.previous
/etc/systemd/system/soil-monitor.service.previous
```

When both backups exist, a failed release automatically attempts to restore them and restart the prior version. The release still exits unsuccessfully so the failure cannot be mistaken for a successful deployment. A failed first deployment reports that automatic rollback is unavailable.

To request the same rollback manually:

```bash
ssh brooks@192.168.1.95 \
  'sudo -n install -o root -g root -m 0755 /usr/local/bin/soil-monitor.previous /usr/local/bin/soil-monitor && sudo -n install -o root -g root -m 0644 /etc/systemd/system/soil-monitor.service.previous /etc/systemd/system/soil-monitor.service && sudo -n systemctl daemon-reload && sudo -n systemctl restart soil-monitor.service && sudo -n systemctl is-active --quiet soil-monitor.service'
```

Inspect a failure directly with:

```bash
ssh brooks@192.168.1.95 'sudo -n systemctl status --no-pager --full soil-monitor.service'
ssh brooks@192.168.1.95 'sudo -n journalctl -u soil-monitor.service -n 50 --no-pager'
```

After the dry run and one-time prerequisites pass, the separate and explicit first live deployment command is:

```bash
mise run release
```

## Rollback

The application owns no persistent readings, so rollback does not affect the broker:

```bash
sudo systemctl disable --now soil-monitor.service
sudo rm /etc/systemd/system/soil-monitor.service
sudo rm /usr/local/bin/soil-monitor
sudo systemctl daemon-reload
```

The environment file can be retained for a later installation or removed separately from `/etc/soil-monitor/soil-monitor.env`.

## Troubleshooting

- **`open SPI port /dev/spidev0.0` fails:** confirm SPI is enabled, reboot, check `/dev/spidev0.0`, and confirm the `soilmonitor` user belongs to the `spi` group.
- **GPIO pin is unavailable or permission is denied:** confirm the service user belongs to `gpio`, no other process owns the pins, and the HAT is seated correctly.
- **The display is blank or shifted:** confirm this is the SH1106 Waveshare OLED HAT in factory four-wire SPI mode, not the similarly named 1.3-inch LCD HAT or a board converted to I2C.
- **Buttons repeat:** increase `SOIL_BUTTON_DEBOUNCE_MS`, then restart the service.
- **`MQTT OFFLINE` remains visible:** confirm the broker URL, broker listener, firewall, and LAN reachability; inspect `journalctl -u soil-monitor.service` for connection errors.
- **Values disappear after restart:** publish retained MQTT messages with `mosquitto_pub -r`; local persistence is outside this MVP.

The MVP assumes an MQTT broker on a trusted LAN. An exposed or untrusted network requires broker authentication, TLS certificate validation, and access controls before deployment.
