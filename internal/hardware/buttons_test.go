package hardware

import (
	"testing"
	"time"
)

func TestAcceptPressDebouncesEdges(t *testing.T) {
	var last time.Time
	start := time.Unix(100, 0)
	if !acceptPress(&last, start, 150*time.Millisecond) {
		t.Fatal("first press was rejected")
	}
	if acceptPress(&last, start.Add(50*time.Millisecond), 150*time.Millisecond) {
		t.Fatal("bounce inside interval was accepted")
	}
	if !acceptPress(&last, start.Add(150*time.Millisecond), 150*time.Millisecond) {
		t.Fatal("press at interval boundary was rejected")
	}
}
