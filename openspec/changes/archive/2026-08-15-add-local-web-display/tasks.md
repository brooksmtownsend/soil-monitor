## 1. Shared State and Configuration

- [x] 1.1 Add `SOIL_HTTP_ADDR` configuration with the `:8080` default, host-and-port validation, environment example, and configuration tests.
- [x] 1.2 Make `core.State` safe for concurrent readers and writers without changing existing OLED navigation or MQTT update behavior.
- [x] 1.3 Add a copied all-plant snapshot model containing plant identifiers, names, latest readings, selected plant, and MQTT connectivity, with state and concurrency tests.

## 2. Web Display Adapter

- [x] 2.1 Add a standard-library HTTP adapter with an embedded `html/template` page and inline styling so no external web assets are required.
- [x] 2.2 Render every configured plant, percentages, unknown placeholders, and online/offline MQTT state from a fresh snapshot on each root request.
- [x] 2.3 Add periodic HTML refresh metadata and verify that later requests reflect updated state without client-side JavaScript.
- [x] 2.4 Reject non-read methods and unknown paths, and verify that configured display names are HTML-escaped.
- [x] 2.5 Implement context-bound serving, bounded graceful shutdown, and clear listener/serving error propagation with adapter tests.

## 3. Application Integration

- [x] 3.1 Add the web server lifecycle port and integrate it into `app.Runner` without changing OLED, button, or MQTT behavior.
- [x] 3.2 Construct the HTTP adapter from the configured address and shared state in the existing `soil-monitor` binary entry point.
- [x] 3.3 Extend runner tests for HTTP startup, coordinated shutdown, and unexpected web-server failure.

## 4. Documentation and Verification

- [x] 4.1 Document the default LAN URL, `SOIL_HTTP_ADDR` override, displayed fields, read-only/trusted-LAN scope, and post-release browser verification.
- [x] 4.2 Run formatting, normal tests, race-enabled tests, vet, shell linting, and the static Linux ARM64 build.
- [x] 4.3 Run a local handler or server smoke test proving the root page responds successfully with representative plant state and no external asset files.
