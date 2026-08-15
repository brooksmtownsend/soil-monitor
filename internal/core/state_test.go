package core

import (
	"strings"
	"sync"
	"testing"
)

func newTestState(t *testing.T) *State {
	t.Helper()
	state, err := NewState([]PlantDefinition{
		{ID: "money-tree", Name: "Money Tree"},
		{ID: "pothos", Name: "Pothos"},
		{ID: "snake-plant", Name: "Snake Plant"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestStateInitialViewAndNavigation(t *testing.T) {
	state := newTestState(t)
	view := state.View()
	if view.PlantID != "money-tree" || view.Position != 1 || view.Total != 3 || view.Moisture != nil || view.Connected {
		t.Fatalf("unexpected initial view: %#v", view)
	}

	state.Previous()
	if got := state.View(); got.PlantID != "snake-plant" || got.Position != 3 {
		t.Fatalf("previous did not wrap: %#v", got)
	}
	state.Next()
	if got := state.View(); got.PlantID != "money-tree" {
		t.Fatalf("next did not wrap: %#v", got)
	}
}

func TestApplyMessageSelectedAndNonSelected(t *testing.T) {
	state := newTestState(t)
	redraw, err := state.ApplyMessage("plants/money-tree/moisture", []byte("47"))
	if err != nil || !redraw {
		t.Fatalf("selected update = %v, %v", redraw, err)
	}
	if got := state.View().Moisture; got == nil || *got != 47 {
		t.Fatalf("selected moisture = %v", got)
	}

	redraw, err = state.ApplyMessage("plants/pothos/moisture", []byte("72"))
	if err != nil || redraw {
		t.Fatalf("non-selected update = %v, %v", redraw, err)
	}
	state.Next()
	if got := state.View().Moisture; got == nil || *got != 72 {
		t.Fatalf("stored non-selected moisture = %v", got)
	}
}

func TestApplyMessageRejectsInvalidInputAndPreservesValue(t *testing.T) {
	state := newTestState(t)
	if _, err := state.ApplyMessage("plants/money-tree/moisture", []byte("60")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		topic   string
		payload string
		wantErr string
	}{
		{topic: "plants/money-tree/moisture", payload: "wet", wantErr: "base-10"},
		{topic: "plants/money-tree/moisture", payload: "101", wantErr: "outside"},
		{topic: "plants/money-tree/moisture", payload: "-1", wantErr: "base-10"},
		{topic: "plants/unknown/moisture", payload: "50", wantErr: "not configured"},
		{topic: "wrong/money-tree/moisture", payload: "50", wantErr: "does not match"},
	}
	for _, tt := range tests {
		if redraw, err := state.ApplyMessage(tt.topic, []byte(tt.payload)); err == nil || redraw || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("ApplyMessage(%q, %q) = %v, %v", tt.topic, tt.payload, redraw, err)
		}
	}
	if got := state.View().Moisture; got == nil || *got != 60 {
		t.Fatalf("invalid input replaced value: %v", got)
	}
}

func TestConnectionStatePreservesMoisture(t *testing.T) {
	state := newTestState(t)
	if _, err := state.ApplyMessage("plants/money-tree/moisture", []byte("33")); err != nil {
		t.Fatal(err)
	}
	if !state.SetConnected(true) || !state.View().Connected {
		t.Fatal("connection state was not updated")
	}
	if !state.SetConnected(false) || state.View().Connected {
		t.Fatal("disconnection state was not updated")
	}
	if got := state.View().Moisture; got == nil || *got != 33 {
		t.Fatalf("connection change lost value: %v", got)
	}
}

func TestSnapshotCopiesAllState(t *testing.T) {
	state := newTestState(t)
	if _, err := state.ApplyMessage("plants/money-tree/moisture", []byte("47")); err != nil {
		t.Fatal(err)
	}
	state.Next()
	state.SetConnected(true)

	snapshot := state.Snapshot()
	if len(snapshot.Plants) != 3 || snapshot.SelectedPlantID != "pothos" || !snapshot.Connected {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Plants[0].Moisture == nil || *snapshot.Plants[0].Moisture != 47 || snapshot.Plants[1].Moisture != nil {
		t.Fatalf("unexpected plant snapshots: %#v", snapshot.Plants)
	}

	*snapshot.Plants[0].Moisture = 99
	snapshot.Plants[0].Name = "Changed"
	fresh := state.Snapshot()
	if fresh.Plants[0].Name != "Money Tree" || fresh.Plants[0].Moisture == nil || *fresh.Plants[0].Moisture != 47 {
		t.Fatalf("snapshot exposed mutable state: %#v", fresh.Plants[0])
	}
}

func TestStateConcurrentSnapshotsAndUpdates(t *testing.T) {
	state := newTestState(t)
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func(offset int) {
			defer workers.Done()
			for i := 0; i < 500; i++ {
				value := byte('0' + (i+offset)%10)
				_, _ = state.ApplyMessage("plants/money-tree/moisture", []byte{value})
				state.SetConnected(i%2 == 0)
				state.Next()
			}
		}(worker)
	}
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 500; i++ {
				snapshot := state.Snapshot()
				if len(snapshot.Plants) != 3 || snapshot.SelectedPlantID == "" {
					t.Errorf("inconsistent snapshot: %#v", snapshot)
					return
				}
				_ = state.View()
			}
		}()
	}
	workers.Wait()
}
