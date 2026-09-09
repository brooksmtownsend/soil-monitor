package history

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"soilmonitor/internal/core"

	_ "modernc.org/sqlite"
)

func TestStoreLifecycleAndQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start := time.Date(2026, 8, 24, 12, 0, 0, 0, time.FixedZone("offset", -4*60*60))
	for i := 0; i < 1005; i++ {
		if err := store.AppendReading(ctx, Reading{PlantID: "money-tree", ObservedAt: start.Add(time.Duration(i) * time.Minute), Raw: 30000 + i, Percentage: i % 101}); err != nil {
			t.Fatalf("append reading %d: %v", i, err)
		}
	}
	if err := store.AppendReading(ctx, Reading{PlantID: "pothos", ObservedAt: start, Raw: 40000, Percentage: 52}); err != nil {
		t.Fatal(err)
	}
	if err := store.RenamePlant(ctx, "money-tree", "Money Tree"); err != nil {
		t.Fatal(err)
	}
	if err := store.RenamePlant(ctx, "unknown", "Unknown"); !errors.Is(err, ErrPlantNotFound) {
		t.Fatalf("rename unknown error = %v", err)
	}

	plants, err := store.Plants(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plants) != 2 || plants[0].ID != "money-tree" || plants[0].Name != "Money Tree" || plants[0].Latest == nil || plants[0].Latest.Raw != 31004 {
		t.Fatalf("plants = %#v", plants)
	}
	if plants[0].FirstSeen.Location() != time.UTC || !plants[0].FirstSeen.Equal(start) {
		t.Fatalf("first seen = %v", plants[0].FirstSeen)
	}

	readings, err := store.Readings(ctx, "money-tree", start.Add(5*time.Minute), start.Add(1004*time.Minute), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 1000 || readings[0].Raw != 30005 || readings[999].Raw != 31004 {
		t.Fatalf("bounded readings: len=%d first=%#v last=%#v", len(readings), readings[0], readings[len(readings)-1])
	}
	if _, err := store.Readings(ctx, "unknown", start, start.Add(time.Hour), 1000); !errors.Is(err, ErrPlantNotFound) {
		t.Fatalf("unknown history error = %v", err)
	}

	readings, err = store.Readings(ctx, "money-tree", start.Add(2*time.Minute), start.Add(4*time.Minute), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 3 || readings[0].Raw != 30002 || readings[2].Raw != 30004 {
		t.Fatalf("inclusive interval readings = %#v", readings)
	}
	empty, err := store.Readings(ctx, "money-tree", start.Add(-2*time.Hour), start.Add(-time.Hour), 1000)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty interval readings = %#v, %v", empty, err)
	}
	for _, tt := range []struct {
		name       string
		start, end time.Time
		limit      int
	}{
		{name: "zero start", end: start, limit: 1},
		{name: "zero end", start: start, limit: 1},
		{name: "reversed", start: start.Add(time.Minute), end: start, limit: 1},
		{name: "zero limit", start: start, end: start.Add(time.Minute)},
	} {
		if _, err := store.Readings(ctx, "money-tree", tt.start, tt.end, tt.limit); err == nil {
			t.Errorf("%s: expected validation error", tt.name)
		}
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	plants, err = reopened.Plants(ctx)
	if err != nil || len(plants) != 2 || plants[0].Name != "Money Tree" {
		t.Fatalf("restored plants = %#v, %v", plants, err)
	}
}

func TestInvalidReadingIsAtomic(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.AppendReading(ctx, Reading{PlantID: "ghost", ObservedAt: time.Now(), Raw: -1, Percentage: 50}); err == nil {
		t.Fatal("expected invalid reading error")
	}
	plants, err := store.Plants(ctx)
	if err != nil || len(plants) != 0 {
		t.Fatalf("plants after failed append = %#v, %v", plants, err)
	}
}

func TestNewerSchemaFailsWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE sentinel(value TEXT); INSERT INTO sentinel VALUES('keep'); PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(path); err == nil {
		store.Close()
		t.Fatal("expected newer schema error")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`SELECT value FROM sentinel`).Scan(&value); err != nil || value != "keep" {
		t.Fatalf("sentinel = %q, %v", value, err)
	}
}

func TestCareSchemaIsAdditiveIdempotentAndVersionOneCompatible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	created := formatTime(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	if _, err := db.Exec(`
        CREATE TABLE plants (id TEXT PRIMARY KEY NOT NULL, name TEXT NULL, first_seen TEXT NOT NULL, last_seen TEXT NOT NULL) STRICT;
        CREATE TABLE readings (id INTEGER PRIMARY KEY AUTOINCREMENT, plant_id TEXT NOT NULL REFERENCES plants(id), observed_at TEXT NOT NULL, raw INTEGER NOT NULL, percentage INTEGER NOT NULL) STRICT;
        INSERT INTO plants(id, name, first_seen, last_seen) VALUES('legacy', 'Legacy', ?, ?);
        INSERT INTO readings(plant_id, observed_at, raw, percentage) VALUES('legacy', ?, 123, 45);
        PRAGMA user_version = 1`, created, created, created); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		store, err := Open(path)
		if err != nil {
			t.Fatalf("open attempt %d: %v", attempt, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, raw int
	var name string
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 1 {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	if err := db.QueryRow(`SELECT COALESCE(p.name, ''), r.raw FROM plants p LEFT JOIN readings r ON r.plant_id = p.id WHERE p.id = 'legacy'`).Scan(&name, &raw); err != nil || name != "Legacy" || raw != 123 {
		t.Fatalf("version-one query = %q, %d, %v", name, raw, err)
	}
	for _, table := range []string{"plant_care", "waterings"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s count = %d, %v", table, count, err)
		}
	}
}

func TestPlantCareStorageLifecycle(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "care.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	registeredAt := time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC)
	jpeg := Photo{Bytes: []byte{0xff, 0xd8, 0xff, 0xe0}, MediaType: "image/jpeg"}
	plant, err := store.RegisterPlant(ctx, "money-tree", "Money Tree", core.CadenceWeekly, registeredAt, &jpeg)
	if err != nil {
		t.Fatal(err)
	}
	if plant.Cadence != core.CadenceWeekly || plant.NextWateringDate != "2026-09-15" || !plant.HasPhoto || plant.Latest != nil {
		t.Fatalf("registered plant = %#v", plant)
	}
	if _, err := store.RegisterPlant(ctx, "money-tree", "Duplicate", core.CadenceWeekly, registeredAt, nil); !errors.Is(err, ErrPlantConflict) {
		t.Fatalf("registration conflict = %v", err)
	}
	storedPhoto, err := store.Photo(ctx, "money-tree")
	if err != nil || storedPhoto.MediaType != jpeg.MediaType || !bytes.Equal(storedPhoto.Bytes, jpeg.Bytes) {
		t.Fatalf("stored photo = %#v, %v", storedPhoto, err)
	}
	if err := store.AppendReading(ctx, Reading{PlantID: "money-tree", ObservedAt: registeredAt.Add(time.Hour), Raw: 32000, Percentage: 47}); err != nil {
		t.Fatal(err)
	}
	plants, err := store.Plants(ctx)
	if err != nil || len(plants) != 1 || plants[0].Latest == nil || plants[0].Latest.Raw != 32000 || plants[0].Cadence != core.CadenceWeekly || !plants[0].HasPhoto {
		t.Fatalf("restored registered plant = %#v, %v", plants, err)
	}
	plant, err = store.SetSchedule(ctx, "money-tree", core.CadenceMonthly, time.Date(2027, 1, 31, 17, 0, 0, 0, time.UTC))
	if err != nil || plant.NextWateringDate != "2027-02-28" {
		t.Fatalf("monthly schedule = %#v, %v", plant, err)
	}
	plant, err = store.ConfirmWatering(ctx, "money-tree", time.Date(2027, 3, 3, 17, 0, 0, 0, time.UTC))
	if err != nil || plant.NextWateringDate != "2027-04-03" {
		t.Fatalf("confirmed watering = %#v, %v", plant, err)
	}
	var wateringCount int
	if err := store.db.QueryRow(`SELECT count(*) FROM waterings WHERE plant_id = 'money-tree'`).Scan(&wateringCount); err != nil || wateringCount != 1 {
		t.Fatalf("watering count = %d, %v", wateringCount, err)
	}
	plant, err = store.SetSchedule(ctx, "money-tree", core.CadenceAsNeeded, registeredAt)
	if err != nil || plant.NextWateringDate != "" {
		t.Fatalf("as-needed schedule = %#v, %v", plant, err)
	}
	if _, err := store.ConfirmWatering(ctx, "money-tree", registeredAt.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	png := Photo{Bytes: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, MediaType: "image/png"}
	if plant, err = store.ReplacePhoto(ctx, "money-tree", png); err != nil || !plant.HasPhoto {
		t.Fatalf("replace photo = %#v, %v", plant, err)
	}
	if _, err := store.ReplacePhoto(ctx, "money-tree", Photo{Bytes: []byte("bad"), MediaType: "image/png"}); err == nil {
		t.Fatal("expected invalid replacement error")
	}
	storedPhoto, err = store.Photo(ctx, "money-tree")
	if err != nil || storedPhoto.MediaType != "image/png" || !bytes.Equal(storedPhoto.Bytes, png.Bytes) {
		t.Fatalf("photo after failed replacement = %#v, %v", storedPhoto, err)
	}
}

func TestUnscheduledWateringAndMissingPlantErrors(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "unscheduled.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC)
	if err := store.AppendReading(ctx, Reading{PlantID: "aloe", ObservedAt: now, Raw: 1, Percentage: 2}); err != nil {
		t.Fatal(err)
	}
	plant, err := store.ConfirmWatering(ctx, "aloe", now)
	if err != nil || plant.Cadence != "" || plant.NextWateringDate != "" {
		t.Fatalf("unscheduled watering = %#v, %v", plant, err)
	}
	if _, err := store.Photo(ctx, "aloe"); !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("missing photo error = %v", err)
	}
	operations := map[string]func() error{
		"schedule": func() error { _, err := store.SetSchedule(ctx, "missing", core.CadenceWeekly, now); return err },
		"watering": func() error { _, err := store.ConfirmWatering(ctx, "missing", now); return err },
		"photo": func() error {
			_, err := store.ReplacePhoto(ctx, "missing", Photo{Bytes: []byte{0xff, 0xd8, 0xff}, MediaType: "image/jpeg"})
			return err
		},
	}
	for operation, call := range operations {
		if err := call(); !errors.Is(err, ErrPlantNotFound) {
			t.Errorf("%s error = %v", operation, err)
		}
	}
}

func TestRegistrationRollsBackWhenCareInsertFails(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "atomic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`CREATE TRIGGER reject_care BEFORE INSERT ON plant_care BEGIN SELECT RAISE(ABORT, 'care unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RegisterPlant(context.Background(), "money-tree", "Money Tree", core.CadenceWeekly, time.Now(), nil); err == nil {
		t.Fatal("expected registration failure")
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM plants WHERE id = 'money-tree'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial registration count = %d, %v", count, err)
	}
}

func TestWateringRollsBackWhenDateUpdateFails(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "watering-atomic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC)
	if _, err := store.RegisterPlant(ctx, "aloe", "Aloe", core.CadenceWeekly, now, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER reject_care_update BEFORE UPDATE ON plant_care BEGIN SELECT RAISE(ABORT, 'care unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmWatering(ctx, "aloe", now.AddDate(0, 0, 9)); err == nil {
		t.Fatal("expected watering failure")
	}
	var count int
	var nextDate string
	if err := store.db.QueryRow(`SELECT count(*) FROM waterings WHERE plant_id = 'aloe'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial watering count = %d, %v", count, err)
	}
	if err := store.db.QueryRow(`SELECT next_watering_date FROM plant_care WHERE plant_id = 'aloe'`).Scan(&nextDate); err != nil || nextDate != "2026-09-15" {
		t.Fatalf("next date after failed watering = %q, %v", nextDate, err)
	}
}
