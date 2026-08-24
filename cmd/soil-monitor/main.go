package main

import (
	"context"
	"fmt"
	"io"
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
	if err := run(logger, os.Args[1:], os.Environ(), os.Stderr); err != nil {
		if config.IsHelp(err) {
			return
		}
		logger.Error("soil monitor stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, args, environ []string, output io.Writer) error {
	cfg, err := config.Load(args, environ, output)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	var displayAdapter core.Display
	var buttons core.ButtonSource
	if cfg.PIHatEnabled {
		if _, err := host.Init(); err != nil {
			return fmt.Errorf("initialize Raspberry Pi hardware: %w", err)
		}
		displayAdapter, err = hardware.NewSH1106(cfg.SPIPort)
		if err != nil {
			return fmt.Errorf("display initialization: %w", err)
		}
		buttons, err = hardware.NewButtons(cfg.Debounce)
		if err != nil {
			_ = displayAdapter.Close()
			return fmt.Errorf("button initialization: %w", err)
		}
	}

	definitions := make([]core.PlantDefinition, len(cfg.Plants))
	for i, plant := range cfg.Plants {
		definitions[i] = core.PlantDefinition{ID: plant.ID, Name: plant.Name}
	}
	state, err := core.NewState(definitions)
	if err != nil {
		closeHardware(buttons, displayAdapter)
		return fmt.Errorf("initialize plant state: %w", err)
	}
	var webAdapter core.WebServer
	if cfg.HTTPEnabled {
		webAdapter, err = webdisplay.New(cfg.HTTPAddr, state)
		if err != nil {
			closeHardware(buttons, displayAdapter)
			return fmt.Errorf("initialize web display: %w", err)
		}
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

func closeHardware(buttons core.ButtonSource, displayAdapter core.Display) {
	if buttons != nil {
		_ = buttons.Close()
	}
	if displayAdapter != nil {
		_ = displayAdapter.Close()
	}
}
