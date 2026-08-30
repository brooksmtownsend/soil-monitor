## MODIFIED Requirements

### Requirement: Plant history page
The browser SHALL provide a server-rendered detail page for each discovered plant containing its identifier, assigned display name or identifier fallback, latest reading, name form, and controls for the rolling history ranges `1d`, `1w`, `1mo`, `3mo`, `6mo`, and `1y`. The selected range SHALL end at the current request time and begin 24 hours, 7 days, 1 calendar month, 3 calendar months, 6 calendar months, or 1 calendar year earlier in UTC, respectively. Calendar subtraction SHALL preserve the UTC time of day and clamp the day to the last valid day of the target month when necessary. The page SHALL render at most the newest 1,000 readings within the selected range, ordered chronologically as an inline SVG percentage chart with an accessible tabular fallback, and SHALL use no external assets or JavaScript dependency.

#### Scenario: Detail page is requested without a range
- **WHEN** a client sends `GET /plants/money-tree` without a `range` query value
- **THEN** the server returns status 200 with `1d` selected and renders only readings from the rolling 24-hour interval ending at the request time

#### Scenario: Known plant has history
- **WHEN** a client sends `GET /plants/money-tree?range=3mo`
- **THEN** the server returns status 200, identifies `3mo` as selected, provides controls for all six supported presets, and renders only readings observed since the instant three calendar months before the request time

#### Scenario: Calendar range crosses a shorter month
- **WHEN** a calendar-month or calendar-year preset targets a month without the request date's day number
- **THEN** the range begins on the target month's final day at the same UTC time of day

#### Scenario: Known plant has more than 1,000 readings
- **WHEN** the selected range contains more than 1,000 stored readings
- **THEN** the chart and table contain only the newest 1,000 readings within that range, ordered from oldest to newest

#### Scenario: Selected range has no readings
- **WHEN** a known plant has no stored readings within the selected range
- **THEN** the page returns status 200, retains the selected range controls, and explicitly reports that no readings exist in that range

#### Scenario: Unsupported range is requested
- **WHEN** a client requests a plant detail page with an empty or unsupported `range` query value
- **THEN** the server returns status 400 without querying or rendering plant history

#### Scenario: Unknown plant is requested
- **WHEN** a detail page is requested for an identifier that has not been discovered
- **THEN** the server returns status 404
