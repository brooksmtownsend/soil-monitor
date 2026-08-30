## MODIFIED Requirements

### Requirement: Historical reading queries
The application SHALL retrieve a requested plant's stored readings in deterministic chronological order, SHALL support an inclusive UTC observation-time interval, and SHALL support a bounded newest-reading query for browser chart rendering. Both time boundaries SHALL be applied before the newest-reading limit.

#### Scenario: Plant detail requests history
- **WHEN** the browser requests a known plant's readings between specified beginning and ending UTC instants with a limit of 1,000
- **THEN** the application returns at most the newest 1,000 readings observed within that interval, ordered from oldest to newest within that result

#### Scenario: Reading occurs at the lower boundary
- **WHEN** a stored reading's observation time equals the requested beginning instant
- **THEN** the application includes that reading in the result

#### Scenario: Reading occurs at the upper boundary
- **WHEN** a stored reading's observation time equals the requested ending instant
- **THEN** the application includes that reading in the result

#### Scenario: Newest limit is applied within the range
- **WHEN** a plant has more than 1,000 readings within the requested interval
- **THEN** the application returns the newest 1,000 qualifying readings rather than readings outside the interval

#### Scenario: Plant has no stored readings
- **WHEN** history is queried for a known plant without readings in the requested interval
- **THEN** the application returns an empty history without fabricating values
