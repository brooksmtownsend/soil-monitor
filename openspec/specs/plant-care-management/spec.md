# plant-care-management Specification

## Purpose

Define durable plant registration and care records, including stable identifiers, identifying photos, watering cadences, and confirmed watering events.

## Requirements

### Requirement: Manual plant registration
The application SHALL register a plant from a valid display name and one of the watering cadences `weekly`, `biweekly`, `monthly`, or `as-needed`. Registration SHALL atomically persist the plant, its cadence, its initial next watering date, and any supplied valid photo before exposing the plant in application views. The display name SHALL contain 1 through 80 Unicode characters after trimming.

#### Scenario: Plant is registered without a photo
- **WHEN** a user registers `Money Tree` with the weekly cadence and no photo on September 8, 2026 in `America/New_York`
- **THEN** the application durably creates the plant with display name `Money Tree`, identifier `money-tree`, and next watering date September 15, 2026

#### Scenario: Plant is registered with a photo
- **WHEN** a user registers a valid plant with a supported photo no larger than 20 MiB
- **THEN** the plant, cadence, initial next watering date, photo bytes, and detected media type are committed together

#### Scenario: Registration persistence fails
- **WHEN** any part of a valid registration cannot be committed
- **THEN** the application exposes no partially registered plant or photo

#### Scenario: Invalid registration is submitted
- **WHEN** the name is empty after trimming, exceeds 80 Unicode characters, produces no valid identifier, or the cadence is unsupported
- **THEN** registration is rejected without changing the catalog

### Requirement: Deterministic plant identifier derivation
The application SHALL derive an immutable kebab-case identifier from the submitted registration name by trimming surrounding whitespace, lowercasing, decomposing Latin letters and removing their combining marks, replacing each remaining run of characters other than ASCII letters and digits with one hyphen, and removing leading or trailing hyphens. The result MUST match `[a-z0-9]+(?:-[a-z0-9]+)*`. Registration SHALL reject a derived identifier that already exists, whether the existing plant was registered through the web interface or discovered through MQTT, and SHALL NOT add a numeric or random suffix.

#### Scenario: Mixed-case name is normalized
- **WHEN** a user registers the name `  Money Tree  `
- **THEN** the application derives the identifier `money-tree` while storing `Money Tree` as the display name

#### Scenario: Latin diacritics are normalized
- **WHEN** a user registers the name `Monstéra Deliciosa`
- **THEN** the application derives the identifier `monstera-deliciosa` while preserving `Monstéra Deliciosa` as the display name

#### Scenario: Separators collapse
- **WHEN** a user registers the name `Ruby's   Plant #2`
- **THEN** the application derives the identifier `ruby-s-plant-2`

#### Scenario: Derived identifier conflicts with an existing plant
- **WHEN** `money-tree` already exists and a user registers `Money Tree`
- **THEN** registration is rejected as a conflict and the existing plant is unchanged

#### Scenario: Name cannot produce an identifier
- **WHEN** normalization removes every character from the submitted name
- **THEN** registration is rejected as invalid and no plant is created

### Requirement: Watering cadence and next date
The application SHALL represent watering due values as calendar dates in `America/New_York`. A weekly cadence SHALL set the next date 7 calendar days after its anchor date, a biweekly cadence SHALL set it 14 calendar days after its anchor date, and a monthly cadence SHALL set it one calendar month after its anchor date while preserving the day of month when possible and otherwise clamping to the target month's final day. An as-needed cadence SHALL have no next watering date. Registration SHALL use its local registration date as the initial anchor.

#### Scenario: Weekly next date is initialized
- **WHEN** a weekly plant is registered on September 8, 2026
- **THEN** its next watering date is September 15, 2026

#### Scenario: Biweekly next date is initialized
- **WHEN** a biweekly plant is registered on September 8, 2026
- **THEN** its next watering date is September 22, 2026

#### Scenario: Monthly next date crosses a shorter month
- **WHEN** a monthly plant is registered on January 31, 2027
- **THEN** its next watering date is February 28, 2027

#### Scenario: As-needed plant is registered
- **WHEN** a plant is registered with the as-needed cadence
- **THEN** it has no next watering date

### Requirement: Watering confirmation
The application SHALL atomically append a watering event with the confirmation instant and update the plant's next watering date using the confirmation's `America/New_York` calendar date as the new anchor. The calculation SHALL use the plant's current cadence and SHALL NOT advance from a previously scheduled date. A plant with no assigned cadence or the as-needed cadence SHALL continue to have no next watering date. A failed persistence operation MUST preserve the previous watering history and next watering date.

#### Scenario: Scheduled plant is watered on its due date
- **WHEN** a weekly plant is confirmed watered on September 15, 2026
- **THEN** a watering event is stored and the next watering date becomes September 22, 2026

#### Scenario: Overdue plant is watered
- **WHEN** a weekly plant due September 15 is confirmed watered on September 18, 2026
- **THEN** a watering event is stored and the next watering date becomes September 25, 2026

#### Scenario: As-needed plant is watered
- **WHEN** an as-needed plant is confirmed watered
- **THEN** a watering event is stored and the plant continues to have no next watering date

#### Scenario: Unscheduled discovered plant is watered
- **WHEN** a plant with no assigned cadence is confirmed watered
- **THEN** a watering event is stored and the plant continues to have no next watering date

#### Scenario: Watering confirmation cannot be stored
- **WHEN** persistence fails while confirming a watering
- **THEN** neither a watering event nor a revised next watering date is committed

### Requirement: Watering schedule maintenance
The application SHALL allow an existing plant's cadence to be assigned or replaced with any supported cadence. Changing to a scheduled cadence SHALL compute the next watering date from the local date of the change, changing to as-needed SHALL clear the next watering date, and the operation SHALL NOT alter previous watering events.

#### Scenario: MQTT-discovered plant receives a schedule
- **WHEN** a plant without a watering cadence is assigned the biweekly cadence on September 8, 2026
- **THEN** its next watering date becomes September 22, 2026

#### Scenario: Plant changes to as-needed
- **WHEN** a scheduled plant is changed to the as-needed cadence
- **THEN** its next watering date is cleared and its watering history is preserved

#### Scenario: Schedule update fails
- **WHEN** a cadence update cannot be persisted
- **THEN** the previous cadence and next watering date remain unchanged

### Requirement: Stored plant photos
The application SHALL accept optional JPEG, PNG, and HEIC plant photos whose file content is no larger than 20 MiB. It SHALL determine the supported media type from the file signature rather than trusting the submitted filename or content type, store the original bytes and detected media type in SQLite, and reject oversized, empty, malformed, or unsupported photo content without storing it. The application SHALL allow a plant's existing photo to be replaced by a later valid upload; a rejected replacement MUST preserve the existing photo.

#### Scenario: JPEG photo is accepted
- **WHEN** a user uploads content with a valid JPEG signature and at most 20 MiB of bytes
- **THEN** the application stores the original bytes with media type `image/jpeg`

#### Scenario: PNG photo is accepted
- **WHEN** a user uploads content with a valid PNG signature and at most 20 MiB of bytes
- **THEN** the application stores the original bytes with media type `image/png`

#### Scenario: HEIC photo is accepted
- **WHEN** a user uploads a valid ISO Base Media File containing a supported HEIC-compatible brand and at most 20 MiB of bytes
- **THEN** the application stores the original bytes with media type `image/heic`

#### Scenario: Declared type does not match content
- **WHEN** a submitted file is named as a supported image but its signature is malformed or unsupported
- **THEN** the application rejects the upload and stores none of its bytes

#### Scenario: Photo exceeds the limit
- **WHEN** a submitted photo contains more than 20 MiB of file content
- **THEN** the application rejects the upload without buffering or storing the complete oversized file

#### Scenario: Existing photo replacement is invalid
- **WHEN** a plant already has a photo and a replacement upload is rejected
- **THEN** the previously stored photo remains available

