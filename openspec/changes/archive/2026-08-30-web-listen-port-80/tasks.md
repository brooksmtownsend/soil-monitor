## 1. Code: Update default HTTP address

- [x] 1.1 In `internal/config/config.go`, change `defaultHTTPAddr` from `:8080` to `:80` and update the help text in the `PrintHelp` function accordingly; verify by running `grep ':80' internal/config/config.go` and confirming no remaining `:8080` references.

- [x] 1.2 In `internal/config/config_test.go`, update the `defaultConfig()` helper's `HTTPAddr` field from `:8080` to `:80`; verify by running `mise run test` and confirming all config tests pass.

## 2. Deployment: Update configuration example and systemd unit

- [x] 2.1 In `deploy/soil-monitor.env.example`, change `SOIL_HTTP_ADDR=:8080` to `SOIL_HTTP_ADDR=:80`; verify file reflects the new default.

- [x] 2.2 In `deploy/soil-monitor.service`, add `AmbientCapabilities=CAP_NET_BIND_SERVICE` and `CapabilityBoundingSet=CAP_NET_BIND_SERVICE` under the `[Service]` section so the `soilmonitor` account can bind port 80 without root; verify the lines are present and correctly placed.

## 3. Documentation: Update README port references

- [x] 3.1 In `README.md`, change all occurrences of `:8080` and `port 8080` to `:80` and `port 80`; update example URLs (e.g. `http://192.168.1.95:8080/` → `http://192.168.1.95/`); add a brief migration note that existing deployments relying on the old default should set `SOIL_HTTP_ADDR=:8080` in `/etc/soil-monitor/soil-monitor.env`.

## 4. OpenSpec: Sync delta spec and validate

- [x] 4.1 Sync the delta spec into the main spec: update the "Configurable HTTP listener" requirement in `openspec/specs/local-web-display/spec.md` to reflect `:80` as the default, matching the MODIFIED requirement in this change's delta spec; verify by running `openspec validate --change web-listen-port-80`.

- [x] 4.2 Run `mise run vet` and `mise run test` to confirm the full suite passes with the new default.
