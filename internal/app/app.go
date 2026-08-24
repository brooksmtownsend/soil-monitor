package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"soilmonitor/internal/core"
	"soilmonitor/internal/history"
)

type renameCommand struct {
	ctx    context.Context
	id     string
	name   string
	result chan error
}

type Commander struct {
	renames chan renameCommand
}

func NewCommander() *Commander {
	return &Commander{renames: make(chan renameCommand)}
}

func (c *Commander) RenamePlant(ctx context.Context, id, name string) error {
	if c == nil {
		return fmt.Errorf("plant command service is unavailable")
	}
	normalized, err := core.NormalizePlantName(name)
	if err != nil {
		return err
	}
	command := renameCommand{ctx: ctx, id: id, name: normalized, result: make(chan error, 1)}
	select {
	case c.renames <- command:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-command.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type Runner struct {
	State    *core.State
	Display  core.Display
	Buttons  core.ButtonSource
	MQTT     core.MQTTSource
	Web      core.WebServer
	History  history.Repository
	Commands *Commander
	Logger   *slog.Logger
	Now      func() time.Time
}

func (r *Runner) Run(ctx context.Context) (runErr error) {
	if r.State == nil || r.MQTT == nil || r.History == nil {
		return fmt.Errorf("state, history, and MQTT adapters are required")
	}
	if (r.Display == nil) != (r.Buttons == nil) {
		return fmt.Errorf("display and buttons must either both be configured or both be omitted")
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	runCtx, cancel := context.WithCancel(ctx)
	var webErrors chan error
	webStarted := false
	webFinished := false
	defer func() {
		cancel()
		if webStarted && !webFinished {
			if webErr := <-webErrors; webErr != nil {
				runErr = errors.Join(runErr, fmt.Errorf("stop web server: %w", webErr))
			}
		}
		runErr = errors.Join(runErr, r.MQTT.Close())
		runErr = errors.Join(runErr, r.History.Close())
		if r.Buttons != nil {
			runErr = errors.Join(runErr, r.Buttons.Close())
		}
		if r.Display != nil {
			runErr = errors.Join(runErr, r.Display.Close())
		}
	}()

	if r.Display != nil {
		if err := r.Display.Render(r.State.View()); err != nil {
			return fmt.Errorf("render startup view: %w", err)
		}
	}
	if err := r.MQTT.Start(runCtx); err != nil {
		return fmt.Errorf("start MQTT: %w", err)
	}
	if r.Web != nil {
		webErrors = make(chan error, 1)
		go func() {
			webErrors <- r.Web.Run(runCtx)
		}()
		webStarted = true
	}

	var buttonEvents <-chan core.Button
	if r.Buttons != nil {
		buttonEvents = r.Buttons.Events()
	}
	mqttEvents := r.MQTT.Events()
	var renameCommands <-chan renameCommand
	if r.Commands != nil {
		renameCommands = r.Commands.renames
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case webErr := <-webErrors:
			webFinished = true
			if webErr == nil {
				return fmt.Errorf("web server stopped unexpectedly")
			}
			return fmt.Errorf("web server: %w", webErr)
		case button, ok := <-buttonEvents:
			if !ok {
				buttonEvents = nil
				continue
			}
			redraw := true
			switch button {
			case core.ButtonPrevious:
				redraw = r.State.Previous()
			case core.ButtonNext:
				redraw = r.State.Next()
			case core.ButtonReserved:
				redraw = false
			default:
				r.Logger.Warn("ignored unknown button", "button", button)
				redraw = false
			}
			if redraw && r.Display != nil {
				if err := r.Display.Render(r.State.View()); err != nil {
					return fmt.Errorf("render button update: %w", err)
				}
			}
		case event, ok := <-mqttEvents:
			if !ok {
				mqttEvents = nil
				continue
			}
			redraw := false
			switch event.Kind {
			case core.MQTTConnected:
				redraw = r.State.SetConnected(true)
			case core.MQTTDisconnected:
				redraw = r.State.SetConnected(false)
			case core.MQTTMessage:
				reading, err := core.ParseReading(event.Topic, event.Payload, r.Now())
				if err != nil {
					r.Logger.Warn("ignored MQTT message", "topic", event.Topic, "error", err)
					break
				}
				err = r.History.AppendReading(runCtx, history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt})
				if err != nil {
					r.Logger.Warn("failed to persist MQTT message", "topic", event.Topic, "error", err)
					break
				}
				redraw = r.State.ApplyReading(reading)
			case core.MQTTError:
				r.Logger.Warn("MQTT adapter error", "error", event.Err)
			default:
				r.Logger.Warn("ignored unknown MQTT event", "kind", event.Kind)
			}
			if redraw && r.Display != nil {
				if err := r.Display.Render(r.State.View()); err != nil {
					return fmt.Errorf("render MQTT update: %w", err)
				}
			}
		case command := <-renameCommands:
			err := r.History.RenamePlant(command.ctx, command.id, command.name)
			redraw := false
			if err == nil {
				redraw = r.State.RenamePlant(command.id, command.name)
			}
			command.result <- err
			if redraw && r.Display != nil {
				if err := r.Display.Render(r.State.View()); err != nil {
					return fmt.Errorf("render plant name update: %w", err)
				}
			}
		}
	}
}
