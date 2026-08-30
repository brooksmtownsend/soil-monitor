## Context

The HTTP listen address is set via `SOIL_HTTP_ADDR` in `internal/config/config.go`. It currently defaults to `:8080`. All references to this default appear in: the config package, its tests, the example env file, and the main spec. See proposal.md for motivation.

## Goals / Non-Goals

**Goals:**
- Change the default value of `SOIL_HTTP_ADDR` from `:8080` to `:80` in code
- Update all documentation and configuration examples to reflect port 80
- Update the delta spec for `local-web-display`

**Non-Goals:**
- Changes to how `SOIL_HTTP_ADDR` is parsed or validated
- Adding TLS, authentication, or any other listener behavior changes
- Changes to sudoers policy (binding to port 80 on Linux as an unprivileged user may require `CAP_NET_BIND_SERVICE`; that is an operator concern addressed in deployment docs, not in this change)

## Decisions

**Single constant change, ripple updates**: The entire change is a one-line default value edit in `config.go` plus coordinated documentation updates. No structural or architectural work is required.

**No `CAP_NET_BIND_SERVICE` automation**: Binding to port 80 on Linux requires elevated privilege for unprivileged processes. The `soilmonitor` service account will need `CAP_NET_BIND_SERVICE` granted via the systemd unit or an `authbind`/`setcap` mechanism. This change documents the requirement and adds the `AmbientCapabilities` line to the systemd unit template; it does not change the sudo policy or bootstrap scripts beyond that.

## Risks / Trade-offs

- **Risk: Existing deployments silently use the old default** → Mitigation: `SOIL_HTTP_ADDR` is in `/etc/soil-monitor/soil-monitor.env` which is host-managed; operators who relied on the `:8080` default without explicitly setting `SOIL_HTTP_ADDR` will need to add `SOIL_HTTP_ADDR=:8080` to preserve the old behavior. Document this in the README migration note.
- **Risk: Port 80 binding fails without elevated privilege** → Mitigation: Add `AmbientCapabilities=CAP_NET_BIND_SERVICE` and `CapabilityBoundingSet=CAP_NET_BIND_SERVICE` to the systemd unit file so the service account can bind low ports.
