package display

import (
	"strings"
	"testing"

	"soilmonitor/internal/core"
)

func TestFormatNoReadingAndDisconnected(t *testing.T) {
	layout := Format(core.View{HasPlant: true, PlantName: "Money Tree", Position: 1, Total: 3})
	if layout.Moisture != "--%" || layout.Position != "1 / 3" || layout.Status != "MQTT OFFLINE" {
		t.Fatalf("unexpected layout: %#v", layout)
	}
}

func TestFormatReadingAndConnected(t *testing.T) {
	layout := Format(core.View{HasPlant: true, PlantName: "Pothos", Latest: &core.Reading{Percentage: 72}, Position: 2, Total: 3, Connected: true})
	if layout.Moisture != "72%" || layout.Status != "" {
		t.Fatalf("unexpected layout: %#v", layout)
	}
}

func TestFormatTruncatesLongNamesDeterministically(t *testing.T) {
	view := core.View{HasPlant: true, PlantName: "An Extremely Long Plant Display Name", Position: 1, Total: 3}
	first := Format(view)
	second := Format(view)
	if first.PlantName != second.PlantName || !strings.HasSuffix(first.PlantName, "…") || len([]rune(first.PlantName)) != maxPlantNameRunes {
		t.Fatalf("unexpected truncated name: %q", first.PlantName)
	}
}

func TestFrameDimensionsAndContent(t *testing.T) {
	frame := Frame(core.View{HasPlant: true, PlantName: "Money Tree", Latest: &core.Reading{Percentage: 47}, Position: 1, Total: 3, Connected: true})
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

func TestFormatEmptyCatalog(t *testing.T) {
	layout := Format(core.View{Connected: true})
	if layout.PlantName != "Waiting for readings" || layout.Position != "0 / 0" || layout.Status != "" {
		t.Fatalf("unexpected empty layout: %#v", layout)
	}
}
