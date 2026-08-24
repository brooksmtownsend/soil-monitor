package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"soilmonitor/internal/core"
)

type Runner struct {
	State   *core.State
	Display core.Display
	Buttons core.ButtonSource
	MQTT    core.MQTTSource
	Web     core.WebServer
	Logger  *slog.Logger
}

func (r *Runner) Run(ctx context.Context) (runErr error) {
	if r.State == nil || r.MQTT == nil {
		return fmt.Errorf("state and MQTT adapters are required")
	}
	if (r.Display == nil) != (r.Buttons == nil) {
		return fmt.Errorf("display and buttons must either both be configured or both be omitted")
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
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
				r.State.Previous()
			case core.ButtonNext:
				r.State.Next()
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
				var err error
				redraw, err = r.State.ApplyMessage(event.Topic, event.Payload)
				if err != nil {
					r.Logger.Warn("ignored MQTT message", "topic", event.Topic, "error", err)
				}
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
		}
	}
}
