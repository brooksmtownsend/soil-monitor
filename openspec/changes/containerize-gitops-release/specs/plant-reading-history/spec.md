# Spec Delta

## ADDED Requirements

### Requirement: Persistent data volume
The deployed service SHALL keep the database on a writable persistent volume that outlives individual containers, and image upgrades or restarts MUST NOT overwrite or remove an existing database.

#### Scenario: Fresh deployment
- **WHEN** the application starts against an empty persistent volume
- **THEN** it creates the database without pre-populated plant data

#### Scenario: Image is upgraded
- **WHEN** a new image version replaces the running one
- **THEN** the existing SQLite database on the volume is preserved and changed only by normal application initialization

## REMOVED Requirements

### Requirement: Persistent data lifecycle
**Reason**: It was defined in terms of bootstrap and routine release operations, which are removed.
**Migration**: Replaced by "Persistent data volume".
