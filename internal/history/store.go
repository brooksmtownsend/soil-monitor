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

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

var ErrPlantNotFound = errors.New("plant not found")

type Reading struct {
	ID         int64
	PlantID    string
	ObservedAt time.Time
	Raw        int
	Percentage int
}

type Plant struct {
	ID        string
	Name      string
	FirstSeen time.Time
	LastSeen  time.Time
	Latest    *Reading
}

type Repository interface {
	Plants(context.Context) ([]Plant, error)
	AppendReading(context.Context, Reading) error
	RenamePlant(context.Context, string, string) error
	Readings(context.Context, string, int) ([]Reading, error)
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
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

func (s *Store) Plants(ctx context.Context) ([]Plant, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT p.id, COALESCE(p.name, ''), p.first_seen, p.last_seen,
               r.id, r.observed_at, r.raw, r.percentage
        FROM plants p
        LEFT JOIN readings r ON r.id = (
            SELECT id FROM readings
            WHERE plant_id = p.id
            ORDER BY observed_at DESC, id DESC LIMIT 1
        )
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
		var observedAt sql.NullString
		if err := rows.Scan(&plant.ID, &plant.Name, &firstSeen, &lastSeen, &readingID, &observedAt, &raw, &percentage); err != nil {
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

func (s *Store) Readings(ctx context.Context, plantID string, limit int) ([]Reading, error) {
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
            FROM readings WHERE plant_id = ?
            ORDER BY observed_at DESC, id DESC LIMIT ?
        ) ORDER BY observed_at, id`, plantID, limit)
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
