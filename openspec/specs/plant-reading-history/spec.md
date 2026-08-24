# plant-reading-history Specification

## Purpose

Define durable local storage, restoration, naming, querying, and deployment lifecycle behavior for discovered plants and moisture readings.

## Requirements

### Requirement: Durable plant catalog
The application SHALL persist every valid plant identifier discovered from an accepted moisture message, its optional user-assigned display name, and its first-seen and last-seen timestamps in the configured local SQLite database. A plant without an assigned display name SHALL use its identifier as its display label.

#### Scenario: First reading discovers a plant
- **WHEN** the first valid reading for `plants/money-tree/moisture` is accepted
- **THEN** the application durably creates the `money-tree` plant record before exposing it in application views

#### Scenario: Discovered plant has no assigned name
- **WHEN** a plant has been discovered and no display name has been assigned
- **THEN** application views use the plant identifier as its display label

#### Scenario: Application restarts
- **WHEN** the application starts with a database containing previously discovered plants
- **THEN** it restores the plant catalog and assigned names without waiting for new MQTT messages

### Requirement: Durable reading history
For every accepted moisture message, the application SHALL atomically ensure the plant exists and append a reading containing the plant identifier, non-negative raw value, percentage from 0 through 100, and application receipt time in UTC before updating the in-memory latest value.

#### Scenario: Valid reading is received
- **WHEN** a valid reading is received for a new or existing plant
- **THEN** its raw value, percentage, and receipt timestamp are durably appended before the latest value changes in application views

#### Scenario: Persistence fails
- **WHEN** the database cannot commit an otherwise valid reading
- **THEN** the application logs the persistence failure and preserves the plant's previously committed latest value

#### Scenario: Latest readings are restored
- **WHEN** the application restarts with historical readings in the database
- **THEN** each discovered plant's most recently stored raw value, percentage, and timestamp are restored into the in-memory view

### Requirement: Historical reading queries
The application SHALL retrieve a requested plant's stored readings in deterministic chronological order and SHALL support a bounded newest-reading query for browser chart rendering.

#### Scenario: Plant detail requests history
- **WHEN** the browser requests a known plant's detail page
- **THEN** the application returns at most the newest 1,000 stored readings ordered from oldest to newest within that result

#### Scenario: Plant has no stored readings
- **WHEN** history is queried for a known plant without readings
- **THEN** the application returns an empty history without fabricating values

### Requirement: Idempotent database initialization
The application SHALL create or migrate the configured SQLite database to the required schema transactionally at startup and SHALL fail startup with a database diagnostic if initialization or restoration cannot complete.

#### Scenario: Database does not exist
- **WHEN** the application starts with a valid path whose database file is absent
- **THEN** it creates the database schema and starts with an empty plant catalog

#### Scenario: Existing schema is current
- **WHEN** the application restarts with a current database schema and stored history
- **THEN** initialization preserves all plant metadata and readings

#### Scenario: Database cannot be opened
- **WHEN** the configured database path cannot be opened or migrated
- **THEN** startup exits non-zero before MQTT, HTTP, or hardware adapters begin normal operation

### Requirement: Persistent data lifecycle
The deployed service SHALL keep the database in a writable persistent state directory owned for the unprivileged service account, and bootstrap and routine release operations MUST NOT overwrite or remove an existing database.

#### Scenario: Fresh host is bootstrapped
- **WHEN** bootstrap prepares a host without an application state directory
- **THEN** it creates a persistent directory writable by `soilmonitor` without pre-populating plant data

#### Scenario: Routine release is performed
- **WHEN** a new application binary and deployment assets are released
- **THEN** the existing SQLite database remains byte-for-byte outside the transferred release artifacts
