package hardware

import (
	"image"
	"image/color"
	"testing"
)

func TestEncodePageUsesSH1106VerticalBits(t *testing.T) {
	frame := image.NewGray(image.Rect(0, 0, 128, 64))
	frame.SetGray(0, 0, color.Gray{Y: 255})
	frame.SetGray(0, 7, color.Gray{Y: 255})
	frame.SetGray(1, 1, color.Gray{Y: 255})
	destination := make([]byte, 128)
	encodePage(destination, frame, 0)
	if destination[0] != 0x81 || destination[1] != 0x02 {
		t.Fatalf("encoded bytes = %#02x %#02x", destination[0], destination[1])
	}
}
