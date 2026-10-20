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
	ID               string
	Name             string
	FirstSeen        time.Time
	LastSeen         time.Time
	Latest           *Reading
	Cadence          Cadence
	NextWateringDate LocalDate
	HasPhoto         bool
}

type PlantSnapshot struct {
	ID               string
	Name             string
	FirstSeen        time.Time
	LastSeen         time.Time
	Latest           *Reading
	Cadence          Cadence
	NextWateringDate LocalDate
	HasPhoto         bool
}

type Snapshot struct {
	Plants    []PlantSnapshot
	Connected bool
}

type State struct {
	mu        sync.RWMutex
	plants    []Plant
	byID      map[string]int
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
		if plant.Cadence != "" && !ValidCadence(plant.Cadence) {
			return nil, fmt.Errorf("plant %q has unsupported watering cadence %q", plant.ID, plant.Cadence)
		}
		if plant.NextWateringDate != "" {
			if _, err := ParseLocalDate(string(plant.NextWateringDate)); err != nil {
				return nil, fmt.Errorf("plant %q: %w", plant.ID, err)
			}
			if plant.Cadence == "" || plant.Cadence == CadenceAsNeeded {
				return nil, fmt.Errorf("plant %q has a next watering date without a scheduled cadence", plant.ID)
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
	return &State{plants: plants, byID: byID}, nil
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

func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plants := make([]PlantSnapshot, len(s.plants))
	for i, plant := range s.plants {
		plants[i] = snapshotPlant(plant)
	}
	return Snapshot{Plants: plants, Connected: s.connected}
}

func (s *State) Plant(id string) (PlantSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index, ok := s.byID[id]
	if !ok {
		return PlantSnapshot{}, false
	}
	plant := s.plants[index]
	return snapshotPlant(plant), true
}

func (s *State) ApplyReading(reading Reading) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.byID[reading.PlantID]
	if !exists {
		index = len(s.plants)
		s.byID[reading.PlantID] = index
		s.plants = append(s.plants, Plant{ID: reading.PlantID, FirstSeen: reading.ObservedAt, LastSeen: reading.ObservedAt})
	}
	s.plants[index].LastSeen = reading.ObservedAt
	s.plants[index].Latest = copyReading(&reading)
	return true
}

func (s *State) RenamePlant(id, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.byID[id]
	if !exists {
		return false
	}
	s.plants[index].Name = name
	return true
}

func (s *State) ApplyPlant(plant Plant) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.byID[plant.ID]
	if !exists {
		index = len(s.plants)
		s.byID[plant.ID] = index
		s.plants = append(s.plants, copyPlant(plant))
		return true
	}
	s.plants[index] = copyPlant(plant)
	return true
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

func snapshotPlant(plant Plant) PlantSnapshot {
	return PlantSnapshot{
		ID:               plant.ID,
		Name:             plantLabel(plant),
		FirstSeen:        plant.FirstSeen,
		LastSeen:         plant.LastSeen,
		Latest:           copyReading(plant.Latest),
		Cadence:          plant.Cadence,
		NextWateringDate: plant.NextWateringDate,
		HasPhoto:         plant.HasPhoto,
	}
}

func copyPlant(plant Plant) Plant { plant.Latest = copyReading(plant.Latest); return plant }
func copyReading(reading *Reading) *Reading {
	if reading == nil {
		return nil
	}
	copied := *reading
	return &copied
}
