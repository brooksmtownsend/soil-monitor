## ADDED Requirements

### Requirement: Eastern Time presentation
The browser SHALL display every human-readable observation timestamp and history-range boundary in the `America/New_York` time zone, using `EST` or `EDT` according to the represented instant. This SHALL apply consistently to initial server-rendered HTML and later WebSocket-driven updates. Machine-readable timestamp values SHALL continue to identify the same absolute instants with RFC 3339 offsets, and persistence and historical filtering SHALL remain based on UTC instants.

#### Scenario: Summer observation is displayed
- **WHEN** a stored observation falls within Eastern daylight-saving time
- **THEN** overview and history views display its local date and time with the `EDT` abbreviation

#### Scenario: Winter observation is displayed
- **WHEN** a stored observation falls within Eastern standard time
- **THEN** overview and history views display its local date and time with the `EST` abbreviation

#### Scenario: Live observation update is received
- **WHEN** a WebSocket snapshot updates an observation timestamp on an open page
- **THEN** the browser displays the same Eastern date, time, and abbreviation that a fresh server-rendered response would display

#### Scenario: History range is rendered
- **WHEN** a plant history page renders the selected range's beginning and ending instants
- **THEN** both human-readable boundaries are displayed in Eastern Time without changing which stored readings qualify for the range

## MODIFIED Requirements

### Requirement: Plant history page
The browser SHALL provide a server-rendered detail page for each discovered plant containing its identifier, assigned display name or identifier fallback, latest reading, name form, and controls for the rolling history ranges `1d`, `1w`, `1mo`, `3mo`, `6mo`, and `1y`. The selected range SHALL end at the current request time and begin 24 hours, 7 days, 1 calendar month, 3 calendar months, 6 calendar months, or 1 calendar year earlier in UTC, respectively. Calendar subtraction SHALL preserve the UTC time of day and clamp the day to the last valid day of the target month when necessary. The page SHALL render at most the newest 1,000 readings within the selected range as an inline SVG percentage chart ordered chronologically from oldest to newest and an accessible table ordered from newest to oldest. Readings with equal observation timestamps SHALL appear in reverse receipt order in the table. The page SHALL use no external assets or JavaScript dependency.

#### Scenario: Detail page is requested without a range
- **WHEN** a client sends `GET /plants/money-tree` without a `range` query value
- **THEN** the server returns status 200 with `1d` selected and renders only readings from the rolling 24-hour interval ending at the request time

#### Scenario: Known plant has history
- **WHEN** a client sends `GET /plants/money-tree?range=3mo`
- **THEN** the server returns status 200, identifies `3mo` as selected, provides controls for all six supported presets, renders the chart from oldest to newest, and renders the table from newest to oldest

#### Scenario: Calendar range crosses a shorter month
- **WHEN** a calendar-month or calendar-year preset targets a month without the request date's day number
- **THEN** the range begins on the target month's final day at the same UTC time of day

#### Scenario: Known plant has more than 1,000 readings
- **WHEN** the selected range contains more than 1,000 stored readings
- **THEN** the chart and table contain only the newest 1,000 readings within that range, with the chart ordered oldest to newest and the table ordered newest to oldest

#### Scenario: Readings share an observation timestamp
- **WHEN** two readings in the selected range have the same observation timestamp
- **THEN** the table shows the reading received later before the reading received earlier

#### Scenario: Selected range has no readings
- **WHEN** a known plant has no stored readings within the selected range
- **THEN** the page returns status 200, retains the selected range controls, and explicitly reports that no readings exist in that range

#### Scenario: Unsupported range is requested
- **WHEN** a client requests a plant detail page with an empty or unsupported `range` query value
- **THEN** the server returns status 400 without querying or rendering plant history

#### Scenario: Unknown plant is requested
- **WHEN** a detail page is requested for an identifier that has not been discovered
- **THEN** the server returns status 404
