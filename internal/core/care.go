package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const MaxPhotoBytes = 20 << 20

type Cadence string

const (
	CadenceWeekly   Cadence = "weekly"
	CadenceBiweekly Cadence = "biweekly"
	CadenceMonthly  Cadence = "monthly"
	CadenceAsNeeded Cadence = "as-needed"
)

type LocalDate string

var careLocation = mustLoadCareLocation("America/New_York")

func ValidCadence(cadence Cadence) bool {
	switch cadence {
	case CadenceWeekly, CadenceBiweekly, CadenceMonthly, CadenceAsNeeded:
		return true
	default:
		return false
	}
}

func NextWateringDate(cadence Cadence, anchor time.Time) (LocalDate, error) {
	if anchor.IsZero() {
		return "", fmt.Errorf("watering date anchor is required")
	}
	if !ValidCadence(cadence) {
		return "", fmt.Errorf("unsupported watering cadence %q", cadence)
	}
	if cadence == CadenceAsNeeded {
		return "", nil
	}
	local := anchor.In(careLocation)
	date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, careLocation)
	switch cadence {
	case CadenceWeekly:
		date = date.AddDate(0, 0, 7)
	case CadenceBiweekly:
		date = date.AddDate(0, 0, 14)
	case CadenceMonthly:
		date = addCalendarMonth(date)
	}
	return LocalDate(date.Format(time.DateOnly)), nil
}

func ParseLocalDate(value string) (LocalDate, error) {
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil || parsed.Format(time.DateOnly) != value {
		return "", fmt.Errorf("invalid local date %q", value)
	}
	return LocalDate(value), nil
}

func DerivePlantID(name string) (string, string, error) {
	displayName, err := NormalizePlantName(name)
	if err != nil {
		return "", "", err
	}
	decomposed := norm.NFD.String(strings.ToLower(displayName))
	var id strings.Builder
	separator := false
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if separator && id.Len() > 0 {
				id.WriteByte('-')
			}
			id.WriteRune(r)
			separator = false
			continue
		}
		separator = true
	}
	derived := id.String()
	if !ValidPlantID(derived) {
		return "", "", fmt.Errorf("plant name must produce a kebab-case identifier")
	}
	return derived, displayName, nil
}

func DetectPhotoMediaType(photo []byte) (string, error) {
	if len(photo) == 0 {
		return "", fmt.Errorf("photo is empty")
	}
	if len(photo) > MaxPhotoBytes {
		return "", fmt.Errorf("photo exceeds %d bytes", MaxPhotoBytes)
	}
	if len(photo) >= 3 && bytes.Equal(photo[:3], []byte{0xff, 0xd8, 0xff}) {
		return "image/jpeg", nil
	}
	if len(photo) >= 8 && bytes.Equal(photo[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png", nil
	}
	if isHEIC(photo) {
		return "image/heic", nil
	}
	return "", fmt.Errorf("photo must be JPEG, PNG, or HEIC")
}

func addCalendarMonth(value time.Time) time.Time {
	target := time.Date(value.Year(), value.Month()+1, 1, 0, 0, 0, 0, value.Location())
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, value.Location()).Day()
	day := value.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(target.Year(), target.Month(), day, 0, 0, 0, 0, value.Location())
}

func isHEIC(photo []byte) bool {
	if len(photo) < 16 || !bytes.Equal(photo[4:8], []byte("ftyp")) {
		return false
	}
	boxSize := uint64(binary.BigEndian.Uint32(photo[:4]))
	headerSize := uint64(8)
	if boxSize == 1 {
		if len(photo) < 24 {
			return false
		}
		boxSize = binary.BigEndian.Uint64(photo[8:16])
		headerSize = 16
	}
	if boxSize < headerSize+8 || boxSize > uint64(len(photo)) {
		return false
	}
	brandStart := headerSize
	if heicBrand(photo[brandStart : brandStart+4]) {
		return true
	}
	for offset := brandStart + 8; offset+4 <= boxSize; offset += 4 {
		if heicBrand(photo[offset : offset+4]) {
			return true
		}
	}
	return false
}

func heicBrand(brand []byte) bool {
	switch string(brand) {
	case "heic", "heix", "hevc", "hevx":
		return true
	default:
		return false
	}
}

func mustLoadCareLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load care time zone %q: %v", name, err))
	}
	return location
}
