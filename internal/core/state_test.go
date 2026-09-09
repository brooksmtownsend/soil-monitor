package core

import (
	"strings"
	"sync"
	"testing"
	"time"
)

var testTime = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func restoredPlant(id, name string, offset int) Plant {
	reading := Reading{PlantID: id, Raw: 30000 + offset, Percentage: 40 + offset, ObservedAt: testTime.Add(time.Duration(offset) * time.Minute)}
	return Plant{ID: id, Name: name, FirstSeen: reading.ObservedAt, LastSeen: reading.ObservedAt, Latest: &reading}
}

func newTestState(t *testing.T) *State {
	t.Helper()
	state, err := NewState([]Plant{restoredPlant("money-tree", "Money Tree", 0), restoredPlant("pothos", "Pothos", 1), restoredPlant("snake-plant", "Snake Plant", 2)})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestEmptyStateAndDynamicNavigation(t *testing.T) {
	state, err := NewState(nil)
	if err != nil {
		t.Fatal(err)
	}
	if view := state.View(); view.HasPlant || view.Total != 0 || state.Next() || state.Previous() {
		t.Fatalf("unexpected empty state: %#v", view)
	}
	if !state.ApplyReading(Reading{PlantID: "money-tree", Raw: 32000, Percentage: 47, ObservedAt: testTime}) {
		t.Fatal("first reading did not request redraw")
	}
	if view := state.View(); !view.HasPlant || view.PlantName != "money-tree" || view.Position != 1 || view.Total != 1 {
		t.Fatalf("unexpected first plant: %#v", view)
	}
	state.ApplyReading(Reading{PlantID: "pothos", Raw: 33000, Percentage: 51, ObservedAt: testTime.Add(time.Minute)})
	if view := state.View(); view.PlantID != "money-tree" || view.Total != 2 {
		t.Fatalf("new plant changed selection: %#v", view)
	}
	state.Previous()
	if view := state.View(); view.PlantID != "pothos" || view.Position != 2 {
		t.Fatalf("previous did not wrap: %#v", view)
	}
}

func TestRestoredOrderAndSnapshotsAreCopied(t *testing.T) {
	aloe := restoredPlant("aloe", "", 0)
	aloe.Cadence = CadenceWeekly
	aloe.NextWateringDate = "2026-08-31"
	aloe.HasPhoto = true
	state, err := NewState([]Plant{restoredPlant("zebra", "", 1), aloe, restoredPlant("basil", "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := state.Snapshot()
	if len(snapshot.Plants) != 3 || snapshot.Plants[0].ID != "aloe" || snapshot.Plants[1].ID != "basil" || snapshot.Plants[2].ID != "zebra" || snapshot.SelectedPlantID != "aloe" {
		t.Fatalf("unexpected restored order: %#v", snapshot)
	}
	snapshot.Plants[0].Name = "changed"
	snapshot.Plants[0].Latest.Percentage = 99
	fresh := state.Snapshot()
	if fresh.Plants[0].Name != "aloe" || fresh.Plants[0].Latest.Percentage == 99 || fresh.Plants[0].Cadence != CadenceWeekly || fresh.Plants[0].NextWateringDate != "2026-08-31" || !fresh.Plants[0].HasPhoto {
		t.Fatalf("snapshot exposed mutable state: %#v", fresh.Plants[0])
	}
}

func TestApplyRegisteredPlantAndThenReading(t *testing.T) {
	state, err := NewState(nil)
	if err != nil {
		t.Fatal(err)
	}
	registeredAt := testTime
	registered := Plant{ID: "money-tree", Name: "Money Tree", FirstSeen: registeredAt, LastSeen: registeredAt, Cadence: CadenceMonthly, NextWateringDate: "2026-09-24", HasPhoto: true}
	if !state.ApplyPlant(registered) {
		t.Fatal("first registered plant did not request redraw")
	}
	if view := state.View(); !view.HasPlant || view.PlantName != "Money Tree" || view.Latest != nil || view.Total != 1 {
		t.Fatalf("registered view = %#v", view)
	}
	reading := Reading{PlantID: registered.ID, Raw: 32000, Percentage: 47, ObservedAt: registeredAt.Add(time.Hour)}
	state.ApplyReading(reading)
	plant, ok := state.Plant(registered.ID)
	if !ok || plant.Latest == nil || plant.Latest.Raw != 32000 || plant.Cadence != CadenceMonthly || plant.NextWateringDate != "2026-09-24" || !plant.HasPhoto {
		t.Fatalf("registered plant after reading = %#v, %v", plant, ok)
	}
	state.ApplyPlant(Plant{ID: "pothos", Name: "Pothos", FirstSeen: registeredAt.Add(time.Minute), LastSeen: registeredAt.Add(time.Minute), Cadence: CadenceAsNeeded})
	if view := state.View(); view.PlantID != "money-tree" || view.Total != 2 {
		t.Fatalf("later registration changed selection: %#v", view)
	}
}

func TestParseReading(t *testing.T) {
	valid, err := ParseReading("plants/money-tree/moisture", []byte(`{"raw":32768,"percentage":47,"future":true}`), testTime)
	if err != nil || valid.PlantID != "money-tree" || valid.Raw != 32768 || valid.Percentage != 47 || valid.ObservedAt.Location() != time.UTC {
		t.Fatalf("valid reading = %#v, %v", valid, err)
	}
	tests := []struct{ topic, payload, want string }{
		{"plants/money-tree/moisture", `47`, "decode"},
		{"plants/money-tree/moisture", `{"raw":1}`, "requires"},
		{"plants/money-tree/moisture", `{"raw":1.5,"percentage":2}`, "unmarshal"},
		{"plants/money-tree/moisture", `{"raw":-1,"percentage":2}`, "non-negative"},
		{"plants/money-tree/moisture", `{"raw":1,"percentage":101}`, "outside"},
		{"plants/Money Tree/moisture", `{"raw":1,"percentage":2}`, "does not match"},
		{"plants/money-tree/moisture", `{"raw":1,"percentage":2} {}`, "trailing"},
	}
	for _, tt := range tests {
		if _, err := ParseReading(tt.topic, []byte(tt.payload), testTime); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParseReading(%q, %q) error = %v, want %q", tt.topic, tt.payload, err, tt.want)
		}
	}
}

func TestRenameAndConnection(t *testing.T) {
	state := newTestState(t)
	if !state.RenamePlant("money-tree", "New Name") || state.View().PlantName != "New Name" {
		t.Fatalf("rename failed: %#v", state.View())
	}
	if state.RenamePlant("unknown", "Unknown") {
		t.Fatal("unknown plant renamed")
	}
	if !state.SetConnected(true) || !state.View().Connected || state.SetConnected(true) {
		t.Fatal("connection transition failed")
	}
}

func TestNormalizePlantName(t *testing.T) {
	if got, err := NormalizePlantName("  Money Tree  "); err != nil || got != "Money Tree" {
		t.Fatalf("normalized name = %q, %v", got, err)
	}
	if _, err := NormalizePlantName(" "); err == nil {
		t.Fatal("expected empty name error")
	}
	if _, err := NormalizePlantName(strings.Repeat("界", 81)); err == nil {
		t.Fatal("expected long name error")
	}
}

func TestStateConcurrentSnapshotsAndUpdates(t *testing.T) {
	state, _ := NewState(nil)
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func(offset int) {
			defer workers.Done()
			for i := 0; i < 500; i++ {
				id := []string{"aloe", "basil", "cactus"}[(i+offset)%3]
				state.ApplyReading(Reading{PlantID: id, Raw: i, Percentage: i % 101, ObservedAt: testTime.Add(time.Duration(i) * time.Second)})
				state.SetConnected(i%2 == 0)
				state.Next()
				state.RenamePlant(id, strings.ToUpper(id))
			}
		}(worker)
	}
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 500; i++ {
				_ = state.Snapshot()
				_ = state.View()
				_, _ = state.Plant("aloe")
			}
		}()
	}
	workers.Wait()
	if len(state.Snapshot().Plants) != 3 {
		t.Fatalf("unexpected plant count: %#v", state.Snapshot())
	}
}
