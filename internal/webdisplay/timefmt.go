package webdisplay

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

const displayTimeLayout = "2006-01-02 15:04:05 MST"

var easternLocation = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load display time zone %q: %v", name, err))
	}
	return location
}

func formatEasternTime(value time.Time) string {
	return value.In(easternLocation).Format(displayTimeLayout)
}

func formatMachineTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
