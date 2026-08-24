package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var plantIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const MaxPlantNameRunes = 80

type Reading struct {
	PlantID    string
	Raw        int
	Percentage int
	ObservedAt time.Time
}

type Plant struct {
	ID        string
	Name      string
	FirstSeen time.Time
	LastSeen  time.Time
	Latest    *Reading
}

type View struct {
	HasPlant  bool
	PlantID   string
	PlantName string
	Latest    *Reading
	Position  int
	Total     int
	Connected bool
}

type PlantSnapshot struct {
	ID        string
	Name      string
	FirstSeen time.Time
	LastSeen  time.Time
	Latest    *Reading
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

func NewState(restored []Plant) (*State, error) {
	plants := make([]Plant, len(restored))
	for i, plant := range restored {
		if !ValidPlantID(plant.ID) {
			return nil, fmt.Errorf("plant %d identifier %q must be kebab-case", i+1, plant.ID)
		}
		if plant.Name != "" {
			if _, err := NormalizePlantName(plant.Name); err != nil {
				return nil, fmt.Errorf("plant %q: %w", plant.ID, err)
			}
		}
		plants[i] = copyPlant(plant)
	}
	sort.SliceStable(plants, func(i, j int) bool {
		if plants[i].FirstSeen.Equal(plants[j].FirstSeen) {
			return plants[i].ID < plants[j].ID
		}
		return plants[i].FirstSeen.Before(plants[j].FirstSeen)
	})
	byID := make(map[string]int, len(plants))
	for i := range plants {
		if _, exists := byID[plants[i].ID]; exists {
			return nil, fmt.Errorf("plant identifier %q is duplicated", plants[i].ID)
		}
		byID[plants[i].ID] = i
	}
	selected := -1
	if len(plants) > 0 {
		selected = 0
	}
	return &State{plants: plants, byID: byID, selected: selected}, nil
}

func ParseReading(topic string, payload []byte, observedAt time.Time) (Reading, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "plants" || parts[2] != "moisture" || !ValidPlantID(parts[1]) {
		return Reading{}, fmt.Errorf("topic %q does not match plants/<kebab-case-plant-id>/moisture", topic)
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return Reading{}, fmt.Errorf("moisture payload is empty")
	}
	var wire struct {
		Raw        *int `json:"raw"`
		Percentage *int `json:"percentage"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&wire); err != nil {
		return Reading{}, fmt.Errorf("decode moisture JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Reading{}, fmt.Errorf("moisture payload contains trailing JSON value")
		}
		return Reading{}, fmt.Errorf("decode trailing moisture JSON: %w", err)
	}
	if wire.Raw == nil || wire.Percentage == nil {
		return Reading{}, fmt.Errorf("moisture payload requires integer raw and percentage fields")
	}
	if *wire.Raw < 0 {
		return Reading{}, fmt.Errorf("raw value %d must be non-negative", *wire.Raw)
	}
	if *wire.Percentage < 0 || *wire.Percentage > 100 {
		return Reading{}, fmt.Errorf("percentage %d is outside 0..100", *wire.Percentage)
	}
	if observedAt.IsZero() {
		return Reading{}, fmt.Errorf("observation time is required")
	}
	return Reading{PlantID: parts[1], Raw: *wire.Raw, Percentage: *wire.Percentage, ObservedAt: observedAt.UTC()}, nil
}

func ValidPlantID(id string) bool { return plantIDPattern.MatchString(id) }

func NormalizePlantName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	count := utf8.RuneCountInString(normalized)
	if count < 1 || count > MaxPlantNameRunes {
		return "", fmt.Errorf("plant name must contain 1 through %d Unicode characters", MaxPlantNameRunes)
	}
	return normalized, nil
}

func (s *State) View() View {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.selected < 0 {
		return View{Connected: s.connected}
	}
	plant := s.plants[s.selected]
	return View{HasPlant: true, PlantID: plant.ID, PlantName: plantLabel(plant), Latest: copyReading(plant.Latest), Position: s.selected + 1, Total: len(s.plants), Connected: s.connected}
}

func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plants := make([]PlantSnapshot, len(s.plants))
	for i, plant := range s.plants {
		plants[i] = PlantSnapshot{ID: plant.ID, Name: plantLabel(plant), FirstSeen: plant.FirstSeen, LastSeen: plant.LastSeen, Latest: copyReading(plant.Latest)}
	}
	selectedID := ""
	if s.selected >= 0 {
		selectedID = s.plants[s.selected].ID
	}
	return Snapshot{Plants: plants, SelectedPlantID: selectedID, Connected: s.connected}
}

func (s *State) Plant(id string) (PlantSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index, ok := s.byID[id]
	if !ok {
		return PlantSnapshot{}, false
	}
	plant := s.plants[index]
	return PlantSnapshot{ID: plant.ID, Name: plantLabel(plant), FirstSeen: plant.FirstSeen, LastSeen: plant.LastSeen, Latest: copyReading(plant.Latest)}, true
}

func (s *State) Previous() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.plants) == 0 {
		return false
	}
	s.selected = (s.selected - 1 + len(s.plants)) % len(s.plants)
	return true
}

func (s *State) Next() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.plants) == 0 {
		return false
	}
	s.selected = (s.selected + 1) % len(s.plants)
	return true
}

func (s *State) ApplyReading(reading Reading) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.byID[reading.PlantID]
	if !exists {
		index = len(s.plants)
		s.byID[reading.PlantID] = index
		s.plants = append(s.plants, Plant{ID: reading.PlantID, FirstSeen: reading.ObservedAt, LastSeen: reading.ObservedAt})
		if s.selected < 0 {
			s.selected = 0
		}
	}
	s.plants[index].LastSeen = reading.ObservedAt
	s.plants[index].Latest = copyReading(&reading)
	return index == s.selected
}

func (s *State) RenamePlant(id, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.byID[id]
	if !exists {
		return false
	}
	s.plants[index].Name = name
	return index == s.selected
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

func plantLabel(plant Plant) string {
	if plant.Name != "" {
		return plant.Name
	}
	return plant.ID
}

func copyPlant(plant Plant) Plant { plant.Latest = copyReading(plant.Latest); return plant }
func copyReading(reading *Reading) *Reading {
	if reading == nil {
		return nil
	}
	copied := *reading
	return &copied
}
