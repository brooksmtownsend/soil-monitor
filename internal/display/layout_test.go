package display

import (
	"strings"
	"testing"

	"soilmonitor/internal/core"
)

func TestFormatNoReadingAndDisconnected(t *testing.T) {
	layout := Format(core.View{PlantName: "Money Tree", Position: 1, Total: 3})
	if layout.Moisture != "--%" || layout.Position != "1 / 3" || layout.Status != "MQTT OFFLINE" {
		t.Fatalf("unexpected layout: %#v", layout)
	}
}

func TestFormatReadingAndConnected(t *testing.T) {
	value := 72
	layout := Format(core.View{PlantName: "Pothos", Moisture: &value, Position: 2, Total: 3, Connected: true})
	if layout.Moisture != "72%" || layout.Status != "" {
		t.Fatalf("unexpected layout: %#v", layout)
	}
}

func TestFormatTruncatesLongNamesDeterministically(t *testing.T) {
	view := core.View{PlantName: "An Extremely Long Plant Display Name", Position: 1, Total: 3}
	first := Format(view)
	second := Format(view)
	if first.PlantName != second.PlantName || !strings.HasSuffix(first.PlantName, "…") || len([]rune(first.PlantName)) != maxPlantNameRunes {
		t.Fatalf("unexpected truncated name: %q", first.PlantName)
	}
}

func TestFrameDimensionsAndContent(t *testing.T) {
	value := 47
	frame := Frame(core.View{PlantName: "Money Tree", Moisture: &value, Position: 1, Total: 3, Connected: true})
	if frame.Bounds().Dx() != Width || frame.Bounds().Dy() != Height {
		t.Fatalf("frame bounds = %v", frame.Bounds())
	}
	nonBlack := false
	for _, pixel := range frame.Pix {
		if pixel != 0 {
			nonBlack = true
			break
		}
	}
	if !nonBlack {
		t.Fatal("rendered frame is blank")
	}
}
