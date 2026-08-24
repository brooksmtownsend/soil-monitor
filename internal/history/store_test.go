package history

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

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

	readings, err := store.Readings(ctx, "money-tree", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 1000 || readings[0].Raw != 30005 || readings[999].Raw != 31004 {
		t.Fatalf("bounded readings: len=%d first=%#v last=%#v", len(readings), readings[0], readings[len(readings)-1])
	}
	if _, err := store.Readings(ctx, "unknown", 1000); !errors.Is(err, ErrPlantNotFound) {
		t.Fatalf("unknown history error = %v", err)
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
