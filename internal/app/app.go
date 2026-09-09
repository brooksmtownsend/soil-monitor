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

type commandKind int

const (
	commandRename commandKind = iota
	commandRegister
	commandSchedule
	commandWater
	commandPhoto
)

type plantCommand struct {
	kind    commandKind
	ctx     context.Context
	id      string
	name    string
	cadence core.Cadence
	photo   *history.Photo
	result  chan commandResult
}

type commandResult struct {
	id  string
	err error
}

type Commander struct {
	commands chan plantCommand
}

func NewCommander() *Commander {
	return &Commander{commands: make(chan plantCommand)}
}

func (c *Commander) RenamePlant(ctx context.Context, id, name string) error {
	normalized, err := core.NormalizePlantName(name)
	if err != nil {
		return err
	}
	if !core.ValidPlantID(id) {
		return fmt.Errorf("invalid plant identifier")
	}
	_, err = c.submit(ctx, plantCommand{kind: commandRename, id: id, name: normalized})
	return err
}

func (c *Commander) RegisterPlant(ctx context.Context, name string, cadence core.Cadence, photo *history.Photo) (string, error) {
	id, normalized, err := core.DerivePlantID(name)
	if err != nil {
		return "", err
	}
	if !core.ValidCadence(cadence) {
		return "", fmt.Errorf("unsupported watering cadence %q", cadence)
	}
	photo, err = copyValidatedPhoto(photo)
	if err != nil {
		return "", err
	}
	return c.submit(ctx, plantCommand{kind: commandRegister, id: id, name: normalized, cadence: cadence, photo: photo})
}

func (c *Commander) SetSchedule(ctx context.Context, id string, cadence core.Cadence) error {
	if !core.ValidPlantID(id) {
		return fmt.Errorf("invalid plant identifier")
	}
	if !core.ValidCadence(cadence) {
		return fmt.Errorf("unsupported watering cadence %q", cadence)
	}
	_, err := c.submit(ctx, plantCommand{kind: commandSchedule, id: id, cadence: cadence})
	return err
}

func (c *Commander) ConfirmWatering(ctx context.Context, id string) error {
	if !core.ValidPlantID(id) {
		return fmt.Errorf("invalid plant identifier")
	}
	_, err := c.submit(ctx, plantCommand{kind: commandWater, id: id})
	return err
}

func (c *Commander) ReplacePhoto(ctx context.Context, id string, photo history.Photo) error {
	if !core.ValidPlantID(id) {
		return fmt.Errorf("invalid plant identifier")
	}
	validated, err := copyValidatedPhoto(&photo)
	if err != nil {
		return err
	}
	_, err = c.submit(ctx, plantCommand{kind: commandPhoto, id: id, photo: validated})
	return err
}

func (c *Commander) submit(ctx context.Context, command plantCommand) (string, error) {
	if c == nil {
		return "", fmt.Errorf("plant command service is unavailable")
	}
	command.ctx = ctx
	command.result = make(chan commandResult, 1)
	select {
	case c.commands <- command:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	select {
	case result := <-command.result:
		return result.id, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func copyValidatedPhoto(photo *history.Photo) (*history.Photo, error) {
	if photo == nil {
		return nil, nil
	}
	mediaType, err := core.DetectPhotoMediaType(photo.Bytes)
	if err != nil {
		return nil, err
	}
	if mediaType != photo.MediaType {
		return nil, fmt.Errorf("photo media type %q does not match detected %q", photo.MediaType, mediaType)
	}
	return &history.Photo{Bytes: append([]byte(nil), photo.Bytes...), MediaType: photo.MediaType}, nil
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
	var commands <-chan plantCommand
	if r.Commands != nil {
		commands = r.Commands.commands
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
			if redraw && r.Web != nil {
				r.Web.Notify(r.State.Snapshot())
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
			if redraw && r.Web != nil {
				r.Web.Notify(r.State.Snapshot())
			}
		case command := <-commands:
			redraw, changed, result := r.applyCommand(command)
			command.result <- result
			if redraw && r.Display != nil {
				if err := r.Display.Render(r.State.View()); err != nil {
					return fmt.Errorf("render plant command update: %w", err)
				}
			}
			if changed && r.Web != nil {
				r.Web.Notify(r.State.Snapshot())
			}
		}
	}
}

func (r *Runner) applyCommand(command plantCommand) (redraw, changed bool, result commandResult) {
	switch command.kind {
	case commandRename:
		result.err = r.History.RenamePlant(command.ctx, command.id, command.name)
		if result.err == nil {
			redraw = r.State.RenamePlant(command.id, command.name)
			changed = true
		}
	case commandRegister:
		var plant history.Plant
		plant, result.err = r.History.RegisterPlant(command.ctx, command.id, command.name, command.cadence, r.Now(), command.photo)
		if result.err == nil {
			redraw = r.State.ApplyPlant(toCorePlant(plant))
			changed = true
			result.id = plant.ID
		}
	case commandSchedule:
		var plant history.Plant
		plant, result.err = r.History.SetSchedule(command.ctx, command.id, command.cadence, r.Now())
		if result.err == nil {
			redraw = r.State.ApplyPlant(toCorePlant(plant))
			changed = true
		}
	case commandWater:
		var plant history.Plant
		plant, result.err = r.History.ConfirmWatering(command.ctx, command.id, r.Now())
		if result.err == nil {
			redraw = r.State.ApplyPlant(toCorePlant(plant))
			changed = true
		}
	case commandPhoto:
		var plant history.Plant
		if command.photo == nil {
			result.err = fmt.Errorf("photo is required")
			break
		}
		plant, result.err = r.History.ReplacePhoto(command.ctx, command.id, *command.photo)
		if result.err == nil {
			redraw = r.State.ApplyPlant(toCorePlant(plant))
			changed = true
		}
	default:
		result.err = fmt.Errorf("unknown plant command %d", command.kind)
	}
	return redraw, changed, result
}

func toCorePlant(plant history.Plant) core.Plant {
	converted := core.Plant{
		ID:               plant.ID,
		Name:             plant.Name,
		FirstSeen:        plant.FirstSeen,
		LastSeen:         plant.LastSeen,
		Cadence:          plant.Cadence,
		NextWateringDate: plant.NextWateringDate,
		HasPhoto:         plant.HasPhoto,
	}
	if plant.Latest != nil {
		converted.Latest = &core.Reading{PlantID: plant.ID, Raw: plant.Latest.Raw, Percentage: plant.Latest.Percentage, ObservedAt: plant.Latest.ObservedAt}
	}
	return converted
}
