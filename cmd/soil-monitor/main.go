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
	"soilmonitor/internal/history"
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
	historyStore, err := history.Open(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("initialize reading history: %w", err)
	}
	defer historyStore.Close()
	restored, err := historyStore.Plants(context.Background())
	if err != nil {
		return fmt.Errorf("restore plant history: %w", err)
	}
	plants := make([]core.Plant, len(restored))
	for i, plant := range restored {
		plants[i] = core.Plant{ID: plant.ID, Name: plant.Name, FirstSeen: plant.FirstSeen, LastSeen: plant.LastSeen}
		if plant.Latest != nil {
			plants[i].Latest = &core.Reading{PlantID: plant.ID, Raw: plant.Latest.Raw, Percentage: plant.Latest.Percentage, ObservedAt: plant.Latest.ObservedAt}
		}
	}
	state, err := core.NewState(plants)
	if err != nil {
		return fmt.Errorf("restore plant state: %w", err)
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

	commands := app.NewCommander()
	var webAdapter core.WebServer
	if cfg.HTTPEnabled {
		webAdapter, err = webdisplay.New(cfg.HTTPAddr, state, historyStore, commands)
		if err != nil {
			closeHardware(buttons, displayAdapter)
			return fmt.Errorf("initialize web display: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runner := app.Runner{
		State:    state,
		Display:  displayAdapter,
		Buttons:  buttons,
		MQTT:     mqttadapter.New(cfg.BrokerURL, cfg.ClientID),
		Web:      webAdapter,
		History:  historyStore,
		Commands: commands,
		Logger:   logger,
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
