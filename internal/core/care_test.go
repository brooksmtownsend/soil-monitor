package core

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func TestDerivePlantID(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantID      string
		wantDisplay string
		wantError   bool
	}{
		{name: "trim and lowercase", input: "  Money Tree  ", wantID: "money-tree", wantDisplay: "Money Tree"},
		{name: "Latin diacritic", input: "Monstéra Deliciosa", wantID: "monstera-deliciosa", wantDisplay: "Monstéra Deliciosa"},
		{name: "separator runs", input: "Ruby's   Plant #2", wantID: "ruby-s-plant-2", wantDisplay: "Ruby's   Plant #2"},
		{name: "empty", input: "   ", wantError: true},
		{name: "non Latin only", input: "界界", wantError: true},
		{name: "too long", input: strings.Repeat("a", MaxPlantNameRunes+1), wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, display, err := DerivePlantID(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("DerivePlantID(%q) = %q, %q, want error", tt.input, id, display)
				}
				return
			}
			if err != nil || id != tt.wantID || display != tt.wantDisplay {
				t.Fatalf("DerivePlantID(%q) = %q, %q, %v", tt.input, id, display, err)
			}
		})
	}
}

func TestNextWateringDate(t *testing.T) {
	tests := []struct {
		name    string
		cadence Cadence
		anchor  time.Time
		want    LocalDate
	}{
		{name: "weekly", cadence: CadenceWeekly, anchor: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), want: "2026-09-15"},
		{name: "biweekly", cadence: CadenceBiweekly, anchor: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), want: "2026-09-22"},
		{name: "overdue confirmation uses actual day", cadence: CadenceWeekly, anchor: time.Date(2026, 9, 18, 16, 0, 0, 0, time.UTC), want: "2026-09-25"},
		{name: "month clamp", cadence: CadenceMonthly, anchor: time.Date(2027, 1, 31, 17, 0, 0, 0, time.UTC), want: "2027-02-28"},
		{name: "leap month clamp", cadence: CadenceMonthly, anchor: time.Date(2028, 1, 31, 17, 0, 0, 0, time.UTC), want: "2028-02-29"},
		{name: "spring DST boundary", cadence: CadenceWeekly, anchor: time.Date(2027, 3, 13, 23, 30, 0, 0, time.UTC), want: "2027-03-20"},
		{name: "as needed", cadence: CadenceAsNeeded, anchor: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NextWateringDate(tt.cadence, tt.anchor)
			if err != nil || got != tt.want {
				t.Fatalf("NextWateringDate(%q, %v) = %q, %v, want %q", tt.cadence, tt.anchor, got, err, tt.want)
			}
		})
	}
	if _, err := NextWateringDate("daily", time.Now()); err == nil {
		t.Fatal("expected unsupported cadence error")
	}
	if _, err := NextWateringDate(CadenceWeekly, time.Time{}); err == nil {
		t.Fatal("expected zero anchor error")
	}
}

func TestParseLocalDate(t *testing.T) {
	for _, value := range []string{"2026-09-08", "2028-02-29", ""} {
		if got, err := ParseLocalDate(value); err != nil || string(got) != value {
			t.Fatalf("ParseLocalDate(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"09/08/2026", "2027-02-29", "2026-9-8"} {
		if _, err := ParseLocalDate(value); err == nil {
			t.Fatalf("ParseLocalDate(%q) accepted invalid date", value)
		}
	}
}

func TestDetectPhotoMediaType(t *testing.T) {
	heic := makeHEIC("mif1", "heic")
	tests := []struct {
		name      string
		photo     []byte
		want      string
		wantError bool
	}{
		{name: "JPEG", photo: []byte{0xff, 0xd8, 0xff, 0xe0}, want: "image/jpeg"},
		{name: "PNG", photo: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, want: "image/png"},
		{name: "HEIC compatible brand", photo: heic, want: "image/heic"},
		{name: "HEIC major brand", photo: makeHEIC("heix"), want: "image/heic"},
		{name: "empty", wantError: true},
		{name: "truncated JPEG", photo: []byte{0xff, 0xd8}, wantError: true},
		{name: "unsupported BMFF", photo: makeHEIC("mif1", "avif"), wantError: true},
		{name: "misleading text", photo: []byte("photo.jpg"), wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectPhotoMediaType(tt.photo)
			if tt.wantError {
				if err == nil {
					t.Fatalf("DetectPhotoMediaType() = %q, want error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("DetectPhotoMediaType() = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
	boundary := make([]byte, MaxPhotoBytes)
	copy(boundary, []byte{0xff, 0xd8, 0xff})
	if got, err := DetectPhotoMediaType(boundary); err != nil || got != "image/jpeg" {
		t.Fatalf("boundary photo = %q, %v", got, err)
	}
	if _, err := DetectPhotoMediaType(append(boundary, 0)); err == nil {
		t.Fatal("expected oversized photo error")
	}
}

func makeHEIC(major string, compatible ...string) []byte {
	size := 16 + 4*len(compatible)
	photo := make([]byte, size)
	binary.BigEndian.PutUint32(photo[:4], uint32(size))
	copy(photo[4:8], "ftyp")
	copy(photo[8:12], major)
	for i, brand := range compatible {
		copy(photo[16+i*4:20+i*4], brand)
	}
	return photo
}
