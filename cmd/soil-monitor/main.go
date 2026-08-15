package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"periph.io/x/host/v3"

	"soilmonitor/internal/app"
	"soilmonitor/internal/config"
	"soilmonitor/internal/core"
	"soilmonitor/internal/hardware"
	"soilmonitor/internal/mqttadapter"
	"soilmonitor/internal/webdisplay"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("soil monitor stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("initialize Raspberry Pi hardware: %w", err)
	}

	displayAdapter, err := hardware.NewSH1106(cfg.SPIPort)
	if err != nil {
		return fmt.Errorf("display initialization: %w", err)
	}
	buttons, err := hardware.NewButtons(cfg.Debounce)
	if err != nil {
		_ = displayAdapter.Close()
		return fmt.Errorf("button initialization: %w", err)
	}

	definitions := make([]core.PlantDefinition, len(cfg.Plants))
	for i, plant := range cfg.Plants {
		definitions[i] = core.PlantDefinition{ID: plant.ID, Name: plant.Name}
	}
	state, err := core.NewState(definitions)
	if err != nil {
		_ = buttons.Close()
		_ = displayAdapter.Close()
		return fmt.Errorf("initialize plant state: %w", err)
	}
	webAdapter, err := webdisplay.New(cfg.HTTPAddr, state)
	if err != nil {
		_ = buttons.Close()
		_ = displayAdapter.Close()
		return fmt.Errorf("initialize web display: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runner := app.Runner{
		State:   state,
		Display: displayAdapter,
		Buttons: buttons,
		MQTT:    mqttadapter.New(cfg.BrokerURL, cfg.ClientID),
		Web:     webAdapter,
		Logger:  logger,
	}
	return runner.Run(ctx)
}
