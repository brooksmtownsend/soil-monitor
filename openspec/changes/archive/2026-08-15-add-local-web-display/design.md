## Context

The soil-monitor process currently owns a single in-memory state model that is updated by MQTT and button events and rendered to the SH1106 OLED. The state is accessed only from the application's event loop, so it has no synchronization. Adding HTTP handlers introduces concurrent reads while keeping the OLED, MQTT contract, and deployment model unchanged.

The target is a Raspberry Pi on a trusted home LAN. The page is operational visibility rather than a general-purpose web application: it needs current values, clear unknown/offline states, and no write operations or frontend toolchain.

## Goals / Non-Goals

**Goals:**

- Serve a useful HTML status page from the existing static Go binary.
- Show all configured plants, their latest in-memory moisture readings, and MQTT connectivity.
- Keep state reads consistent and race-free while MQTT and button events continue.
- Use only the Go standard library for HTTP, templates, and graceful shutdown.
- Make listener configuration and failures explicit and testable.

**Non-Goals:**

- Historical graphs, persistence, calibration, alerts, or editing plant configuration.
- Browser controls for OLED selection or MQTT publication.
- JavaScript, WebSockets, server-sent events, external assets, or a frontend build.
- Authentication, TLS termination, internet exposure, or reverse-proxy provisioning.
- A separate web process or binary.

## Decisions

### Render HTML on each request from a state snapshot

The root handler will obtain an immutable snapshot containing all plants, copied moisture values, the selected plant identifier, and MQTT connectivity, then render it through `html/template`. This keeps the page aligned with current in-memory state and ensures configured names are escaped. Exposing internal state pointers or duplicating state in the web package was rejected because either approach risks races or divergence.

### Make core state safe for concurrent readers

`core.State` will protect its mutable fields with a read/write mutex. Existing mutation methods will take the write lock, while OLED `View` and the new all-plant `Snapshot` method will take the read lock and return copies. An event-loop-only proxy was considered, but it would require request/response coordination between every HTTP handler and the runner for little benefit at this scale.

### Use a server-rendered page with HTML refresh

The response will contain inline CSS and a standard refresh directive so the browser requests a fresh snapshot periodically. Unknown readings will render as `--`, and MQTT state will be labeled online or offline. JavaScript polling and streaming updates were rejected because a short refresh interval is sufficient for slowly changing moisture data and keeps the artifact genuinely static and inspectable.

### Use one standard-library HTTP server

An internal web adapter will own an `http.Server`, a pre-parsed template, and the read-only snapshot source. It will serve `GET /` and reject unsupported methods or paths through standard HTTP responses. All markup and styling will be compiled into Go source, preserving the single-binary deployment.

### Configure the listener through `SOIL_HTTP_ADDR`

The new setting will default to `:8080`, making the page reachable from the Pi and other LAN devices at `http://192.168.1.95:8080/`. Startup configuration will reject malformed listen addresses. Operators can bind to loopback or another port by overriding the value.

### Tie HTTP lifetime to the application context

The runner will start the HTTP server alongside MQTT processing, monitor its result, and treat unexpected listener or serving failures as process errors. Context cancellation will initiate bounded graceful shutdown so systemd stops do not abandon active responses. HTTP startup must not require the broker to be connected; an offline page remains useful.

## Risks / Trade-offs

- [Concurrent HTTP reads race with MQTT updates] → Synchronize `core.State`, return copied snapshots, and run the test suite with the race detector.
- [Binding `:8080` exposes status to the LAN] → Keep the interface read-only, document the trusted-LAN assumption, and allow loopback binding through configuration.
- [A refresh serves slightly stale data] → Accept a short bounded delay because soil moisture changes slowly and avoiding client-side code is a goal.
- [HTTP listener failure restarts an otherwise functional OLED process] → Fail clearly and let systemd restart; silently losing a configured interface would hide an operational problem.
- [Inline HTML becomes difficult to evolve] → Keep the page deliberately small and covered by handler tests; a richer frontend requires a separate future change.

## Migration Plan

1. Add synchronized snapshots, HTTP configuration, the web adapter, and lifecycle integration with automated tests.
2. Add `SOIL_HTTP_ADDR=:8080` to the environment example and the Pi's host-managed environment file.
3. Release the existing single binary and verify the OLED, MQTT updates, and `http://192.168.1.95:8080/` together.
4. Roll back by deploying the prior binary; the additional environment variable is harmless to older versions.

## Open Questions

None. The initial page is intentionally read-only and LAN-scoped.
