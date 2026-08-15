package display

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"soilmonitor/internal/core"
)

const (
	Width             = 128
	Height            = 64
	maxPlantNameRunes = 18
)

type Layout struct {
	PlantName string
	Moisture  string
	Position  string
	Status    string
}

func Format(view core.View) Layout {
	moisture := "--%"
	if view.Moisture != nil {
		moisture = fmt.Sprintf("%d%%", *view.Moisture)
	}
	status := ""
	if !view.Connected {
		status = "MQTT OFFLINE"
	}
	return Layout{
		PlantName: truncate(view.PlantName, maxPlantNameRunes),
		Moisture:  moisture,
		Position:  fmt.Sprintf("%d / %d", view.Position, view.Total),
		Status:    status,
	}
}

func Frame(view core.View) *image.Gray {
	layout := Format(view)
	frame := image.NewGray(image.Rect(0, 0, Width, Height))
	drawer := font.Drawer{Dst: frame, Src: image.NewUniform(color.Gray{Y: 255}), Face: basicfont.Face7x13}
	drawText := func(text string, x, baseline int) {
		drawer.Dot = fixed.P(x, baseline)
		drawer.DrawString(text)
	}

	drawText(layout.PlantName, 1, 12)
	drawText(layout.Moisture, centeredX(layout.Moisture), 34)
	drawText(layout.Position, centeredX(layout.Position), 49)
	if layout.Status != "" {
		drawText(layout.Status, centeredX(layout.Status), 63)
	}
	return frame
}

func truncate(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}

func centeredX(text string) int {
	width := font.MeasureString(basicfont.Face7x13, text).Ceil()
	if width >= Width {
		return 0
	}
	return (Width - width) / 2
}
