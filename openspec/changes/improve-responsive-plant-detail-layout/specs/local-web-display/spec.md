## ADDED Requirements

### Requirement: Responsive plant detail tile layout
The plant detail page SHALL organize current moisture, display-name editing, photo display and replacement, watering controls, and moisture history into visually distinct tiles. The compact tiles SHALL resize into a three-column arrangement when the viewport can accommodate it, a two-column arrangement at intermediate widths, and a single vertically scrolling column on narrow mobile viewports. The moisture-history tile SHALL span the full available row in multi-column arrangements, and no tile, form control, chart, or history table SHALL force horizontal page scrolling at a supported viewport width.

#### Scenario: Plant detail page is viewed on a wide screen
- **WHEN** the available viewport width can comfortably accommodate three compact tiles
- **THEN** the page presents compact plant information and controls in a three-column arrangement and presents moisture history across the full grid width

#### Scenario: Plant detail page is viewed at an intermediate width
- **WHEN** the available viewport width can accommodate two compact tiles but not three
- **THEN** the compact tiles reflow into a two-column arrangement and moisture history continues to occupy the full available row

#### Scenario: Plant detail page is viewed on a narrow mobile screen
- **WHEN** the available viewport width cannot comfortably accommodate two compact tiles
- **THEN** every tile appears in a single vertical reading order that can be reached by normal page scrolling without horizontal page scrolling

#### Scenario: Plant detail content is grouped into tiles
- **WHEN** a known plant's detail page is rendered
- **THEN** current moisture, display-name editing, photo display and replacement, watering controls, and moisture history each appear within an identifiable tile while retaining their existing content and actions

#### Scenario: Historical readings exceed the visible tile area
- **WHEN** the moisture-history table is wider or taller than the immediately visible content area
- **THEN** the history tile keeps the page layout intact and allows the user to reach the chart, table columns, and later rows without clipping content
