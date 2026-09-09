## Context

See `proposal.md` for the motivation and `specs/local-web-display/spec.md` for the responsive behavior contract. The embedded web UI currently keeps all markup and CSS in `internal/webdisplay/server.go`. The overview already uses a responsive card grid, but the detail page renders its photo, header reading, care forms, and history panels as unrelated full-width blocks. The page must remain server-rendered, dependency-free, usable without JavaScript, and compatible with the existing WebSocket selectors and form routes.

The in-progress `add-web-plant-registration-and-watering` change owns the underlying name, photo, and watering behavior. This change treats that behavior as an input and changes only the detail page's visual grouping and responsive presentation.

## Goals / Non-Goals

**Goals:**

- Establish an explicit detail-page grid and stable tile hierarchy without changing the overview grid.
- Make each tile shrink safely by constraining grid children and form controls, while containing wide history content within the history tile.
- Preserve semantic headings, labels, alternative photo text, chart description, table caption, source order, and no-JavaScript operation.
- Keep the styling within the embedded binary and reuse the existing color palette and card vocabulary.

**Non-Goals:**

- Redesigning the homepage, registration page, chart data, or history range behavior.
- Adding client-side layout logic, a CSS framework, external fonts, images, or other assets.
- Changing plant-care persistence, commands, endpoints, WebSocket payloads, or form submission flows.
- Introducing collapsible or draggable widgets, user-saved layouts, or independent vertical scrolling inside compact tiles.

## Decisions

### Use a detail-specific explicit CSS grid

Wrap the detail tiles in a dedicated grid container and use three equal `minmax(0, 1fr)` tracks at wide widths, two tracks at an intermediate breakpoint, and one track at the existing mobile-scale breakpoint. Give the history tile `grid-column: 1 / -1` so it remains the visually dominant, full-row section in either multi-column arrangement. Use `min-width: 0` on tiles so long controls and table content cannot enlarge a grid track.

Explicit responsive tracks are preferred over copying the overview's fully automatic `auto-fit` rule because the requested three-to-two-to-one hierarchy and full-width history placement should be predictable. JavaScript-driven measurement was rejected because layout belongs in CSS and the page must remain useful without JavaScript.

### Give each user task one semantic tile

Keep the back navigation and concise page heading outside the tile grid. Inside the grid, create separate semantic sections for current moisture, display name, photo management, watering, and moisture history. The current-moisture tile contains percentage, raw reading, observation time, and an explicit unknown state. The photo tile combines the existing image, when present, with the add-or-replace form. The name and watering tiles retain their existing forms. Source order will prioritize current status and care actions before the verbose history section and will also become the mobile reading order.

Keeping photo presentation and replacement together avoids a decorative image floating outside its related action. Combining all controls into a single generic settings tile was rejected because it would recreate the current clutter and make mobile scanning harder.

### Apply polish through scoped, reusable visual rules

Continue using the existing `.panel` foundation for border, background, and radius, then add detail-specific classes for consistent gaps, heading margins, card-level alignment, subdued metadata, and compact form spacing. Constrain the detail photo to its tile, keep a stable aspect ratio with `object-fit: cover`, and let actions align naturally without stretching buttons on wide screens. Existing narrow-screen form controls may fill their tile width to preserve touch usability.

Detail-specific selectors are preferred over broad changes to `.plant`, `.panel`, or all forms because those shared rules also affect the working homepage and registration page.

### Contain history overflow at the content boundary

Keep the chart fluid with its existing view box. Wrap the readings table in a focusable or otherwise accessible horizontal overflow region within the history tile when the table cannot shrink enough; the page itself must not scroll horizontally. Do not cap tile height or create nested vertical scrolling, so all readings remain reachable through normal page scrolling.

Allowing the entire page to overflow was rejected because it makes every tile difficult to use on mobile. Hiding columns or readings was rejected because it would weaken the existing accessible-table contract.

### Verify generated markup and responsive contracts without new dependencies

Extend the existing HTTP template tests to assert the five tile groupings, unchanged form actions and live-update hooks, a full-width history class, three/two/one-column rules, shrink constraints, and contained table overflow. Continue using the repository's existing Go test and vet tasks; visual resizing remains a focused manual check because the project has no browser-test dependency.

## Risks / Trade-offs

- **[A three-column grid can make content feel cramped near its breakpoint]** → Choose breakpoints based on the existing 60rem content width and minimum usable form widths, then manually inspect just above and below each breakpoint.
- **[Shared CSS edits could unintentionally alter the homepage or registration form]** → Scope new layout and polish selectors beneath detail-page classes and retain existing shared defaults.
- **[A wide table introduces horizontal scrolling inside one tile]** → Keep overflow local to the table wrapper, preserve every column, and avoid nested vertical scrolling.
- **[Source order may not match the most attractive desktop placement]** → Treat mobile reading order and keyboard flow as authoritative, using CSS placement only where it does not make focus order surprising.
- **[Static template tests cannot prove visual quality]** → Pair precise markup and CSS assertions with manual checks at representative desktop, tablet, and phone widths.

## Migration Plan

Deploy the template and CSS changes with the normal single-binary release after tests and responsive checks pass. No data, configuration, or asset migration is required. Rollback consists of deploying the prior binary; stored plant metadata, photos, schedules, and readings remain compatible.
