package core

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type PlantDefinition struct {
	ID   string
	Name string
}

type Plant struct {
	ID       string
	Name     string
	Moisture *int
}

type View struct {
	PlantID   string
	PlantName string
	Moisture  *int
	Position  int
	Total     int
	Connected bool
}

type PlantSnapshot struct {
	ID       string
	Name     string
	Moisture *int
}

type Snapshot struct {
	Plants          []PlantSnapshot
	SelectedPlantID string
	Connected       bool
}

type State struct {
	mu        sync.RWMutex
	plants    []Plant
	byID      map[string]int
	selected  int
	connected bool
}

func NewState(definitions []PlantDefinition) (*State, error) {
	if len(definitions) == 0 {
		return nil, fmt.Errorf("at least one plant is required")
	}
	plants := make([]Plant, len(definitions))
	byID := make(map[string]int, len(definitions))
	for i, definition := range definitions {
		if definition.ID == "" || definition.Name == "" {
			return nil, fmt.Errorf("plant %d requires an identifier and name", i+1)
		}
		if _, exists := byID[definition.ID]; exists {
			return nil, fmt.Errorf("plant identifier %q is duplicated", definition.ID)
		}
		plants[i] = Plant{ID: definition.ID, Name: definition.Name}
		byID[definition.ID] = i
	}
	return &State{plants: plants, byID: byID}, nil
}

func (s *State) View() View {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plant := s.plants[s.selected]
	return View{
		PlantID:   plant.ID,
		PlantName: plant.Name,
		Moisture:  copyInt(plant.Moisture),
		Position:  s.selected + 1,
		Total:     len(s.plants),
		Connected: s.connected,
	}
}

func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plants := make([]PlantSnapshot, len(s.plants))
	for i, plant := range s.plants {
		plants[i] = PlantSnapshot{
			ID:       plant.ID,
			Name:     plant.Name,
			Moisture: copyInt(plant.Moisture),
		}
	}
	return Snapshot{
		Plants:          plants,
		SelectedPlantID: s.plants[s.selected].ID,
		Connected:       s.connected,
	}
}

func (s *State) Previous() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selected = (s.selected - 1 + len(s.plants)) % len(s.plants)
}

func (s *State) Next() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selected = (s.selected + 1) % len(s.plants)
}

func (s *State) SetConnected(connected bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected == connected {
		return false
	}
	s.connected = connected
	return true
}

// ApplyMessage stores a valid moisture message. The bool reports whether the
// currently selected plant changed and therefore needs to be redrawn.
func (s *State) ApplyMessage(topic string, payload []byte) (bool, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "plants" || parts[1] == "" || parts[2] != "moisture" {
		return false, fmt.Errorf("topic %q does not match plants/<plant-id>/moisture", topic)
	}
	if len(payload) == 0 {
		return false, fmt.Errorf("moisture payload is empty")
	}
	for _, char := range payload {
		if char < '0' || char > '9' {
			return false, fmt.Errorf("moisture payload %q is not a base-10 integer", payload)
		}
	}
	value, err := strconv.Atoi(string(payload))
	if err != nil {
		return false, fmt.Errorf("moisture payload %q is not a base-10 integer: %w", payload, err)
	}
	if value < 0 || value > 100 {
		return false, fmt.Errorf("moisture value %d is outside 0..100", value)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.byID[parts[1]]
	if !exists {
		return false, fmt.Errorf("plant %q is not configured", parts[1])
	}
	s.plants[index].Moisture = &value
	return index == s.selected, nil
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
