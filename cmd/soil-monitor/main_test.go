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
	if !bytes.Contains(output.Bytes(), []byte("-database-path")) {
		t.Fatalf("unexpected help output:\n%s", output.String())
	}
}

func TestRunDatabaseFailurePrecedesAdapterInitialization(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(logger, []string{"--mqtt-broker=tcp://127.0.0.1:1883", "--database-path=/dev/null/soil.db", "--http=false"}, nil, io.Discard)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("initialize reading history")) {
		t.Fatalf("run() error = %v", err)
	}
}
