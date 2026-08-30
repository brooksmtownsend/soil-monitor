## Why

The web dashboard defaults to port 8080, which requires users to append `:8080` to the URL. Changing the default to port 80 lets operators and users access the dashboard at plain `http://<hostname>/` without remembering a non-standard port.

## What Changes

- The default value of `SOIL_HTTP_ADDR` changes from `:8080` to `:80`
- All references to port 8080 in configuration examples, documentation, and spec scenarios are updated to port 80

## Capabilities

### New Capabilities
<!-- None -->

### Modified Capabilities

- `local-web-display`: The configurable HTTP listener's default address changes from `:8080` to `:80`; scenarios and documentation referencing port 8080 must reflect port 80.

## Impact

- `internal/config/config.go`: default value for `HTTPAddr`
- `internal/config/config_test.go`: test expectations for default address
- `deploy/soil-monitor.env.example`: example address value
- `README.md`: quickstart URL and port references
- `openspec/specs/local-web-display/spec.md`: default port in requirements and scenarios
