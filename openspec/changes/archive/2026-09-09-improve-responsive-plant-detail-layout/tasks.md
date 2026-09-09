## 1. Detail Tile Structure

- [x] 1.1 Extend plant detail template tests to require separate current-moisture, display-name, photo-management, watering, and moisture-history tiles while retaining existing form actions, accessibility content, and live-update selectors; verify the focused `internal/webdisplay` tests fail for the missing structure and pass after implementation.
- [x] 1.2 Restructure the plant detail template into the tested semantic tile order, combine the displayed photo with its add-or-replace form, and preserve explicit known and unknown reading states; verify a rendered known-plant response contains all five tiles and the existing detail-page behavior tests pass.

## 2. Responsive Layout and Polish

- [x] 2.1 Add test assertions for a detail-specific three-, two-, and one-column grid, a full-row history tile, shrink-safe grid children and controls, and locally contained history-table overflow; verify the focused template test covers every responsive contract from the delta spec.
- [x] 2.2 Add scoped detail-page CSS for the responsive grid, tile spacing and hierarchy, heading and metadata rhythm, form alignment, constrained photo presentation, fluid chart, and history overflow wrapper; verify the generated page satisfies the CSS assertions without changing the overview or registration markup.
- [x] 2.3 Resize the rendered plant detail page at representative wide desktop, intermediate tablet or narrow desktop, and mobile widths; verify it presents three, two, and one compact columns respectively, keeps history full-width in multi-column layouts, preserves logical keyboard and reading order, and introduces no page-level horizontal scrolling.

## 3. Verification

- [x] 3.1 Run `mise run test` and verify all Go and deployment fixture tests pass.
- [x] 3.2 Run `mise run vet` and verify static analysis passes.
- [x] 3.3 Run strict validation for `improve-responsive-plant-detail-layout` and verify the OpenSpec change passes before synchronizing or archiving it.
