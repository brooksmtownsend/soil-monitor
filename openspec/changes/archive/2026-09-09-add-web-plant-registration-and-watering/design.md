## Context

See `proposal.md` for motivation and the delta specs for behavior. Today, `plants` is created only as part of appending a reading, `core.State` holds lightweight plant data restored at startup, the application runner serializes mutations, and the web adapter receives a narrow rename command interface. SQLite schema version 1 contains `plants` and `readings`; the installed rollback binary rejects a higher `PRAGMA user_version`. The web server is an unauthenticated trusted-LAN service embedded in the static Go binary and runs on a memory-constrained Raspberry Pi.

## Goals / Non-Goals

**Goals:**

- Keep registration, care updates, and MQTT ingestion within the existing repository/state/runner architecture.
- Make every multi-field mutation transactional and expose it to in-memory views only after persistence succeeds.
- Bound upload work before reading an entire maliciously large request while retaining original accepted image bytes in SQLite.
- Keep the additive database changes readable by the currently installed binary so failed-release rollback can still restart it.

**Non-Goals:**

- Image editing, resizing, thumbnail generation, metadata extraction, HEIC transcoding, or guaranteeing HEIC rendering in browsers that do not support it.
- Plant deletion, photo deletion, custom cadence intervals, watering reminders or notifications, and a watering-history UI.
- Authentication, TLS, or exposure of the trusted-LAN web service to untrusted networks.
- Changing a plant identifier when its display name changes.

## Decisions

### Store care data in additive companion tables

Add `plant_care` as a one-to-zero-or-one companion to `plants`, with nullable cadence and next-date fields plus nullable photo bytes and media type. Add `waterings` with an auto-incrementing receipt-order key, plant foreign key, and UTC confirmation instant. Scheduled registration creates both the `plants` and `plant_care` rows in one transaction; MQTT discovery continues to create only `plants` until care data is assigned. This keeps existing `plants` and `readings` layouts intact and represents migrated or MQTT-only plants naturally as unscheduled plants without photos.

The new tables will be created idempotently alongside the existing version-1 migration without raising `PRAGMA user_version`. This unusual choice is deliberate: the previous binary accepts schema version 1 and ignores unknown tables, so automatic binary rollback remains operational after the new binary has started. Raising the version to 2 was rejected because a failed activation after migration would cause the rollback binary to reject the database, violating the current failed-release recovery contract. Adding care columns directly to `plants` was rejected because idempotent column migration is harder and couples large photo rows to common catalog queries.

Dates are stored as canonical `YYYY-MM-DD` values, while watering event instants use the repository's UTC timestamp format. Database checks constrain cadence strings, require cadence/next-date consistency, and require photo bytes/media type to be both present or both absent. Repository reads select only a `has_photo` existence flag into catalog snapshots; the potentially 20 MiB blob is loaded only by the photo endpoint.

### Treat `America/New_York` dates as domain values

Central care helpers convert the command instant to `America/New_York`, discard time of day, and add 7 days, 14 days, or one calendar month. Monthly calculation constructs the target month and clamps to its last valid day. Recomputing from the actual confirmation or schedule-change date, rather than the previous due date, prevents an overdue plant from remaining overdue after it is watered. Keeping dates separate from instants avoids daylight-saving shifts changing the displayed due day.

### Derive identifiers once at registration

A core normalization helper trims and validates the existing 1–80-rune display name, lowercases it, applies Unicode canonical decomposition, drops combining marks, converts maximal non-ASCII-alphanumeric runs to a single hyphen, and trims boundary hyphens. `golang.org/x/text/unicode/norm` will provide deterministic decomposition; the resulting identifier is checked with the existing plant-ID validator. The repository insert is the final uniqueness authority, mapping a primary-key conflict to a domain conflict error. Prechecking state improves the common response but is not relied upon for correctness.

The display name remains independently editable through the existing route, and edits never rerun identifier derivation. Transliteration of non-Latin scripts was rejected because there is no unambiguous locale-independent mapping; a name that produces no ASCII identifier is returned to the form as invalid.

### Use typed serialized application commands

Extend the commander/runner boundary with registration, schedule, watering, and photo commands rather than allowing HTTP handlers to mutate core state. Each runner case calls a repository transaction, then applies the returned committed plant snapshot to `core.State`, refreshes the OLED only when its visible fields changed, and broadcasts the new web snapshot. A single typed command envelope or separate command channels are both compatible, but one envelope is preferred to avoid adding one select branch and shutdown edge case per mutation.

Photo reads and historical reads remain repository queries outside the mutation loop because they do not mutate state. Core snapshots carry cadence, next date, and `HasPhoto`, never photo bytes. This keeps routine snapshot copies and WebSocket messages small.

### Bound and validate multipart photos at the HTTP boundary

Wrap registration and replacement bodies in `http.MaxBytesReader` with enough fixed allowance for required multipart fields in addition to the 20 MiB photo limit. Parse multipart form data with a small in-memory threshold, remove any temporary parts after the request, and copy the photo through a `20 MiB + 1 byte` limited reader. A single-slot upload semaphore bounds concurrent large upload processing on the Pi. Oversized requests or fields map to 413 before a command is submitted.

Detect PNG and JPEG from their required byte signatures. Detect HEIC by parsing the leading ISO Base Media File `ftyp` box and recognizing documented HEIC-compatible major or compatible brands; do not trust the extension or submitted `Content-Type`. The original bytes and canonical `image/jpeg`, `image/png`, or `image/heic` type are stored. Full decode validation and HEIC conversion were rejected because they add substantial native or image-codec complexity to the static ARM64 deliverable.

Serve photos through a dedicated GET/HEAD route with the stored media type, explicit content length, `X-Content-Type-Options: nosniff`, and `Cache-Control: no-store`. Templates use that URL only when `HasPhoto` is true. HEIC will render where the browser supports it; documentation recommends JPEG for broad compatibility.

### Keep forms server-rendered and use explicit HTTP outcomes

Add a registration page and multipart create route, plus detail-page POST routes for schedule, watering, and photo replacement. Successful commands use 303 redirects to the canonical detail page, preserving the existing post/redirect/get style. Validation errors use 400, identifier conflicts 409, oversized uploads 413, unsupported image content 415, unknown plants 404, and storage failures 500. The existing history range query is preserved on reads; mutations redirect to the default detail range.

## Risks / Trade-offs

- **[Large photos increase database size and backup time]** → Cap each photo at 20 MiB, avoid loading blobs in catalog queries, document that photos share the existing SQLite backup lifecycle, and leave resizing as future work.
- **[Concurrent uploads could exhaust Pi memory or temporary storage]** → Limit body and field reads, use a small multipart memory threshold, clean temporary files, and serialize upload processing with a one-slot gate.
- **[HEIC may not render in every browser]** → Preserve and serve valid HEIC accurately, state the limitation in the form and operator docs, and recommend JPEG when portability matters.
- **[Signature checks do not prove an image fully decodes]** → Use strict supported signatures and nosniff response headers; full codec decoding is outside scope and malformed content cannot execute as HTML under the photo route.
- **[Keeping additive tables at schema version 1 weakens version-number signaling]** → Document the compatibility migration in code, test creation on both empty and existing version-1 databases, and reserve a version bump for a future migration that also provides database-aware rollback.
- **[Twenty-megabyte SQLite writes can briefly delay readings]** → Keep the existing single-connection consistency model and surface failures; the upload gate and trusted-LAN scope bound contention, while MQTT events remain buffered by their adapter.

## Migration Plan

1. On startup, retain the current version checks and version-1 base migration, then create `plant_care` and `waterings` with `IF NOT EXISTS` in the same migration transaction.
2. Do not backfill existing plants. They restore with no cadence, no next date, and no photo until edited in the browser; existing names and readings remain unchanged.
3. Deploy through the normal release workflow. If activation fails, the previous binary can restart against the additive tables because `PRAGMA user_version` remains 1 and the original tables are unchanged.
4. Rolling back loses no care data: the previous binary ignores the companion tables, and a later deployment of the new binary reads them again.
