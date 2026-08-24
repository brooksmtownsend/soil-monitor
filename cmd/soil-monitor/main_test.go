package main

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"soilmonitor/internal/config"
)

func TestRunHelpStopsBeforeAdapterInitialization(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(logger, []string{"--help"}, nil, &output)
	if !config.IsHelp(err) {
		t.Fatalf("run() error = %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("--pi-hat=false")) {
		t.Fatalf("unexpected help output:\n%s", output.String())
	}
}
