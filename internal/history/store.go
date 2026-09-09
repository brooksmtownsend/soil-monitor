package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"soilmonitor/internal/core"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

var (
	ErrPlantNotFound = errors.New("plant not found")
	ErrPlantConflict = errors.New("plant identifier already exists")
	ErrPhotoNotFound = errors.New("plant photo not found")
)

type Reading struct {
	ID         int64
	PlantID    string
	ObservedAt time.Time
	Raw        int
	Percentage int
}

type Plant struct {
	ID               string
	Name             string
	FirstSeen        time.Time
	LastSeen         time.Time
	Latest           *Reading
	Cadence          core.Cadence
	NextWateringDate core.LocalDate
	HasPhoto         bool
}

type Photo struct {
	Bytes     []byte
	MediaType string
}

type Repository interface {
	Plants(context.Context) ([]Plant, error)
	AppendReading(context.Context, Reading) error
	RenamePlant(context.Context, string, string) error
	RegisterPlant(context.Context, string, string, core.Cadence, time.Time, *Photo) (Plant, error)
	SetSchedule(context.Context, string, core.Cadence, time.Time) (Plant, error)
	ConfirmWatering(context.Context, string, time.Time) (Plant, error)
	ReplacePhoto(context.Context, string, Photo) (Plant, error)
	Photo(context.Context, string) (Photo, error)
	Readings(context.Context, string, time.Time, time.Time, int) ([]Reading, error)
	Close() error
}

type Store struct {
	db        *sql.DB
	closeOnce sync.Once
	closeErr  error
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	dsn := (&url.URL{Scheme: "file", Path: filepath.Clean(path), RawQuery: "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read database schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version < 1 {
		statements := []string{
			`CREATE TABLE plants (
                id TEXT PRIMARY KEY NOT NULL,
                name TEXT NULL,
                first_seen TEXT NOT NULL,
                last_seen TEXT NOT NULL
            ) STRICT`,
			`CREATE TABLE readings (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                plant_id TEXT NOT NULL REFERENCES plants(id),
                observed_at TEXT NOT NULL,
                raw INTEGER NOT NULL CHECK(raw >= 0),
                percentage INTEGER NOT NULL CHECK(percentage BETWEEN 0 AND 100)
            ) STRICT`,
			`CREATE INDEX readings_plant_observed_idx ON readings(plant_id, observed_at, id)`,
			`PRAGMA user_version = 1`,
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply database schema version 1: %w", err)
			}
		}
	}
	compatibleStatements := []string{
		`CREATE TABLE IF NOT EXISTS plant_care (
            plant_id TEXT PRIMARY KEY NOT NULL REFERENCES plants(id),
            cadence TEXT NULL CHECK(cadence IN ('weekly', 'biweekly', 'monthly', 'as-needed')),
            next_watering_date TEXT NULL,
            photo BLOB NULL CHECK(photo IS NULL OR length(photo) BETWEEN 1 AND 20971520),
            photo_media_type TEXT NULL CHECK(photo_media_type IN ('image/jpeg', 'image/png', 'image/heic')),
            CHECK(
                (cadence IS NULL AND next_watering_date IS NULL) OR
                (cadence = 'as-needed' AND next_watering_date IS NULL) OR
                (cadence IN ('weekly', 'biweekly', 'monthly') AND next_watering_date IS NOT NULL)
            ),
            CHECK((photo IS NULL) = (photo_media_type IS NULL))
        ) STRICT`,
		`CREATE TABLE IF NOT EXISTS waterings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            plant_id TEXT NOT NULL REFERENCES plants(id),
            watered_at TEXT NOT NULL
        ) STRICT`,
		`CREATE INDEX IF NOT EXISTS waterings_plant_watered_idx ON waterings(plant_id, watered_at, id)`,
	}
	for _, statement := range compatibleStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply compatible care schema: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

func (s *Store) Plants(ctx context.Context) ([]Plant, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT p.id, COALESCE(p.name, ''), p.first_seen, p.last_seen,
		       r.id, r.observed_at, r.raw, r.percentage,
               c.cadence, c.next_watering_date, c.photo IS NOT NULL
        FROM plants p
        LEFT JOIN readings r ON r.id = (
            SELECT id FROM readings
            WHERE plant_id = p.id
            ORDER BY observed_at DESC, id DESC LIMIT 1
        )
		LEFT JOIN plant_care c ON c.plant_id = p.id
        ORDER BY p.first_seen, p.id`)
	if err != nil {
		return nil, fmt.Errorf("query plants: %w", err)
	}
	defer rows.Close()

	var plants []Plant
	for rows.Next() {
		var plant Plant
		var firstSeen, lastSeen string
		var readingID, raw, percentage sql.NullInt64
		var observedAt, cadence, nextDate sql.NullString
		if err := rows.Scan(&plant.ID, &plant.Name, &firstSeen, &lastSeen, &readingID, &observedAt, &raw, &percentage, &cadence, &nextDate, &plant.HasPhoto); err != nil {
			return nil, fmt.Errorf("scan plant: %w", err)
		}
		plant.FirstSeen, err = parseTime(firstSeen)
		if err != nil {
			return nil, fmt.Errorf("parse first-seen time for %q: %w", plant.ID, err)
		}
		plant.LastSeen, err = parseTime(lastSeen)
		if err != nil {
			return nil, fmt.Errorf("parse last-seen time for %q: %w", plant.ID, err)
		}
		if readingID.Valid {
			observed, parseErr := parseTime(observedAt.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parse latest reading time for %q: %w", plant.ID, parseErr)
			}
			plant.Latest = &Reading{ID: readingID.Int64, PlantID: plant.ID, ObservedAt: observed, Raw: int(raw.Int64), Percentage: int(percentage.Int64)}
		}
		if err := assignCare(&plant, cadence, nextDate); err != nil {
			return nil, err
		}
		plants = append(plants, plant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plants: %w", err)
	}
	return plants, nil
}

func (s *Store) AppendReading(ctx context.Context, reading Reading) error {
	if reading.PlantID == "" || reading.Raw < 0 || reading.Percentage < 0 || reading.Percentage > 100 || reading.ObservedAt.IsZero() {
		return fmt.Errorf("invalid reading")
	}
	observedAt := formatTime(reading.ObservedAt)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reading transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO plants(id, first_seen, last_seen) VALUES(?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET last_seen = excluded.last_seen`, reading.PlantID, observedAt, observedAt); err != nil {
		return fmt.Errorf("upsert plant %q: %w", reading.PlantID, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO readings(plant_id, observed_at, raw, percentage) VALUES(?, ?, ?, ?)`, reading.PlantID, observedAt, reading.Raw, reading.Percentage); err != nil {
		return fmt.Errorf("insert reading for %q: %w", reading.PlantID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reading for %q: %w", reading.PlantID, err)
	}
	return nil
}

func (s *Store) RenamePlant(ctx context.Context, id, name string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE plants SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("rename plant %q: %w", id, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check renamed plant %q: %w", id, err)
	}
	if count == 0 {
		return ErrPlantNotFound
	}
	return nil
}

func (s *Store) RegisterPlant(ctx context.Context, id, name string, cadence core.Cadence, registeredAt time.Time, photo *Photo) (Plant, error) {
	if !core.ValidPlantID(id) || name == "" || !core.ValidCadence(cadence) || registeredAt.IsZero() {
		return Plant{}, fmt.Errorf("invalid plant registration")
	}
	nextDate, err := core.NextWateringDate(cadence, registeredAt)
	if err != nil {
		return Plant{}, err
	}
	if err := validatePhoto(photo); err != nil {
		return Plant{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Plant{}, fmt.Errorf("begin plant registration: %w", err)
	}
	defer tx.Rollback()
	instant := formatTime(registeredAt)
	result, err := tx.ExecContext(ctx, `INSERT INTO plants(id, name, first_seen, last_seen) VALUES(?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, id, name, instant, instant)
	if err != nil {
		return Plant{}, fmt.Errorf("insert plant %q: %w", id, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Plant{}, fmt.Errorf("check inserted plant %q: %w", id, err)
	}
	if count == 0 {
		return Plant{}, ErrPlantConflict
	}
	var photoBytes any
	var photoType any
	if photo != nil {
		photoBytes = photo.Bytes
		photoType = photo.MediaType
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO plant_care(plant_id, cadence, next_watering_date, photo, photo_media_type) VALUES(?, ?, NULLIF(?, ''), ?, ?)`, id, cadence, nextDate, photoBytes, photoType); err != nil {
		return Plant{}, fmt.Errorf("insert care data for %q: %w", id, err)
	}
	plant, err := queryPlant(ctx, tx, id)
	if err != nil {
		return Plant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Plant{}, fmt.Errorf("commit plant registration for %q: %w", id, err)
	}
	return plant, nil
}

func (s *Store) SetSchedule(ctx context.Context, id string, cadence core.Cadence, changedAt time.Time) (Plant, error) {
	if !core.ValidPlantID(id) || !core.ValidCadence(cadence) || changedAt.IsZero() {
		return Plant{}, fmt.Errorf("invalid watering schedule")
	}
	nextDate, err := core.NextWateringDate(cadence, changedAt)
	if err != nil {
		return Plant{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Plant{}, fmt.Errorf("begin schedule update: %w", err)
	}
	defer tx.Rollback()
	if err := requirePlant(ctx, tx, id); err != nil {
		return Plant{}, err
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO plant_care(plant_id, cadence, next_watering_date) VALUES(?, ?, NULLIF(?, ''))
        ON CONFLICT(plant_id) DO UPDATE SET cadence = excluded.cadence, next_watering_date = excluded.next_watering_date`, id, cadence, nextDate); err != nil {
		return Plant{}, fmt.Errorf("set watering schedule for %q: %w", id, err)
	}
	plant, err := queryPlant(ctx, tx, id)
	if err != nil {
		return Plant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Plant{}, fmt.Errorf("commit watering schedule for %q: %w", id, err)
	}
	return plant, nil
}

func (s *Store) ConfirmWatering(ctx context.Context, id string, wateredAt time.Time) (Plant, error) {
	if !core.ValidPlantID(id) || wateredAt.IsZero() {
		return Plant{}, fmt.Errorf("invalid watering confirmation")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Plant{}, fmt.Errorf("begin watering confirmation: %w", err)
	}
	defer tx.Rollback()
	if err := requirePlant(ctx, tx, id); err != nil {
		return Plant{}, err
	}
	var cadenceValue sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT cadence FROM plant_care WHERE plant_id = ?`, id).Scan(&cadenceValue); errors.Is(err, sql.ErrNoRows) {
		cadenceValue = sql.NullString{}
	} else if err != nil {
		return Plant{}, fmt.Errorf("read watering cadence for %q: %w", id, err)
	}
	var nextDate core.LocalDate
	if cadenceValue.Valid {
		nextDate, err = core.NextWateringDate(core.Cadence(cadenceValue.String), wateredAt)
		if err != nil {
			return Plant{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO waterings(plant_id, watered_at) VALUES(?, ?)`, id, formatTime(wateredAt)); err != nil {
		return Plant{}, fmt.Errorf("insert watering for %q: %w", id, err)
	}
	if cadenceValue.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE plant_care SET next_watering_date = NULLIF(?, '') WHERE plant_id = ?`, nextDate, id); err != nil {
			return Plant{}, fmt.Errorf("advance watering date for %q: %w", id, err)
		}
	}
	plant, err := queryPlant(ctx, tx, id)
	if err != nil {
		return Plant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Plant{}, fmt.Errorf("commit watering for %q: %w", id, err)
	}
	return plant, nil
}

func (s *Store) ReplacePhoto(ctx context.Context, id string, photo Photo) (Plant, error) {
	if !core.ValidPlantID(id) {
		return Plant{}, fmt.Errorf("invalid plant identifier")
	}
	if err := validatePhoto(&photo); err != nil {
		return Plant{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Plant{}, fmt.Errorf("begin photo replacement: %w", err)
	}
	defer tx.Rollback()
	if err := requirePlant(ctx, tx, id); err != nil {
		return Plant{}, err
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO plant_care(plant_id, photo, photo_media_type) VALUES(?, ?, ?)
        ON CONFLICT(plant_id) DO UPDATE SET photo = excluded.photo, photo_media_type = excluded.photo_media_type`, id, photo.Bytes, photo.MediaType); err != nil {
		return Plant{}, fmt.Errorf("replace photo for %q: %w", id, err)
	}
	plant, err := queryPlant(ctx, tx, id)
	if err != nil {
		return Plant{}, err
	}
	if err := tx.Commit(); err != nil {
		return Plant{}, fmt.Errorf("commit photo replacement for %q: %w", id, err)
	}
	return plant, nil
}

func (s *Store) Photo(ctx context.Context, id string) (Photo, error) {
	if !core.ValidPlantID(id) {
		return Photo{}, ErrPlantNotFound
	}
	var photo Photo
	err := s.db.QueryRowContext(ctx, `SELECT photo, photo_media_type FROM plant_care WHERE plant_id = ? AND photo IS NOT NULL`, id).Scan(&photo.Bytes, &photo.MediaType)
	if errors.Is(err, sql.ErrNoRows) {
		var exists int
		if findErr := s.db.QueryRowContext(ctx, `SELECT 1 FROM plants WHERE id = ?`, id).Scan(&exists); errors.Is(findErr, sql.ErrNoRows) {
			return Photo{}, ErrPlantNotFound
		} else if findErr != nil {
			return Photo{}, fmt.Errorf("find plant %q: %w", id, findErr)
		}
		return Photo{}, ErrPhotoNotFound
	}
	if err != nil {
		return Photo{}, fmt.Errorf("read photo for %q: %w", id, err)
	}
	return photo, nil
}

func (s *Store) Readings(ctx context.Context, plantID string, start, end time.Time, limit int) ([]Reading, error) {
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("reading interval must have non-zero bounds")
	}
	if start.After(end) {
		return nil, fmt.Errorf("reading interval start must not be after end")
	}
	if limit < 1 {
		return nil, fmt.Errorf("reading limit must be positive")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM plants WHERE id = ?`, plantID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPlantNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find plant %q: %w", plantID, err)
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, observed_at, raw, percentage FROM (
            SELECT id, observed_at, raw, percentage
            FROM readings
            WHERE plant_id = ? AND observed_at >= ? AND observed_at <= ?
            ORDER BY observed_at DESC, id DESC LIMIT ?
        ) ORDER BY observed_at, id`, plantID, formatTime(start), formatTime(end), limit)
	if err != nil {
		return nil, fmt.Errorf("query readings for %q: %w", plantID, err)
	}
	defer rows.Close()
	var readings []Reading
	for rows.Next() {
		var reading Reading
		var observedAt string
		reading.PlantID = plantID
		if err := rows.Scan(&reading.ID, &observedAt, &reading.Raw, &reading.Percentage); err != nil {
			return nil, fmt.Errorf("scan reading for %q: %w", plantID, err)
		}
		reading.ObservedAt, err = parseTime(observedAt)
		if err != nil {
			return nil, fmt.Errorf("parse reading time for %q: %w", plantID, err)
		}
		readings = append(readings, reading)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate readings for %q: %w", plantID, err)
	}
	return readings, nil
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() { s.closeErr = s.db.Close() })
	return s.closeErr
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryPlant(ctx context.Context, queryer queryRower, id string) (Plant, error) {
	var plant Plant
	var firstSeen, lastSeen string
	var readingID, raw, percentage sql.NullInt64
	var observedAt, cadence, nextDate sql.NullString
	err := queryer.QueryRowContext(ctx, `
        SELECT p.id, COALESCE(p.name, ''), p.first_seen, p.last_seen,
               r.id, r.observed_at, r.raw, r.percentage,
               c.cadence, c.next_watering_date, c.photo IS NOT NULL
        FROM plants p
        LEFT JOIN readings r ON r.id = (
            SELECT id FROM readings WHERE plant_id = p.id ORDER BY observed_at DESC, id DESC LIMIT 1
        )
        LEFT JOIN plant_care c ON c.plant_id = p.id
        WHERE p.id = ?`, id).Scan(&plant.ID, &plant.Name, &firstSeen, &lastSeen, &readingID, &observedAt, &raw, &percentage, &cadence, &nextDate, &plant.HasPhoto)
	if errors.Is(err, sql.ErrNoRows) {
		return Plant{}, ErrPlantNotFound
	}
	if err != nil {
		return Plant{}, fmt.Errorf("query plant %q: %w", id, err)
	}
	plant.FirstSeen, err = parseTime(firstSeen)
	if err != nil {
		return Plant{}, fmt.Errorf("parse first-seen time for %q: %w", id, err)
	}
	plant.LastSeen, err = parseTime(lastSeen)
	if err != nil {
		return Plant{}, fmt.Errorf("parse last-seen time for %q: %w", id, err)
	}
	if readingID.Valid {
		observed, parseErr := parseTime(observedAt.String)
		if parseErr != nil {
			return Plant{}, fmt.Errorf("parse latest reading time for %q: %w", id, parseErr)
		}
		plant.Latest = &Reading{ID: readingID.Int64, PlantID: id, ObservedAt: observed, Raw: int(raw.Int64), Percentage: int(percentage.Int64)}
	}
	if err := assignCare(&plant, cadence, nextDate); err != nil {
		return Plant{}, err
	}
	return plant, nil
}

func assignCare(plant *Plant, cadence, nextDate sql.NullString) error {
	if cadence.Valid {
		plant.Cadence = core.Cadence(cadence.String)
		if !core.ValidCadence(plant.Cadence) {
			return fmt.Errorf("plant %q has invalid stored cadence %q", plant.ID, cadence.String)
		}
	}
	if nextDate.Valid {
		parsed, err := core.ParseLocalDate(nextDate.String)
		if err != nil {
			return fmt.Errorf("plant %q: %w", plant.ID, err)
		}
		plant.NextWateringDate = parsed
	}
	return nil
}

func requirePlant(ctx context.Context, queryer queryRower, id string) error {
	var exists int
	if err := queryer.QueryRowContext(ctx, `SELECT 1 FROM plants WHERE id = ?`, id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return ErrPlantNotFound
	} else if err != nil {
		return fmt.Errorf("find plant %q: %w", id, err)
	}
	return nil
}

func validatePhoto(photo *Photo) error {
	if photo == nil {
		return nil
	}
	mediaType, err := core.DetectPhotoMediaType(photo.Bytes)
	if err != nil {
		return err
	}
	if photo.MediaType != mediaType {
		return fmt.Errorf("photo media type %q does not match detected %q", photo.MediaType, mediaType)
	}
	return nil
}
