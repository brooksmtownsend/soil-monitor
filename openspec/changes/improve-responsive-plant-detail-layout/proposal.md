## Why

The plant detail page presents useful care and moisture data as a long sequence of loosely grouped sections, which feels cluttered and makes poor use of tablet and desktop space. A responsive tile layout modeled on the overview page will create a clearer hierarchy while remaining comfortable to scan and operate on mobile devices.

## What Changes

- Arrange plant detail information and controls into a responsive tile grid that generally presents three columns on wide screens, two columns at intermediate widths, and one vertically scrolling column on narrow screens.
- Present current moisture, display-name editing, photo display and replacement, and watering controls as compact tiles.
- Give moisture history the available full grid width so its range controls, chart, and reading table remain the primary verbose content area.
- Polish spacing, typography, control alignment, card treatment, image sizing, and responsive overflow while following the overview page's established visual language.
- Preserve server-rendered behavior, progressive enhancement, form actions, history semantics, and live moisture and watering updates.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `local-web-display`: Define the responsive layout and content hierarchy required for the plant detail page across wide, intermediate, and narrow viewports.

## Impact

- Affects the embedded plant detail template and shared styles in `internal/webdisplay/server.go`.
- Extends web display tests to verify semantic tile grouping, responsive grid rules, full-width history treatment, and narrow-screen overflow safeguards.
- Does not change HTTP routes, stored data, application state, external assets, dependencies, or JavaScript behavior.
