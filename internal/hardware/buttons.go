package hardware

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"

	"soilmonitor/internal/core"
)

type buttonBinding struct {
	name   string
	button core.Button
	pin    gpio.PinIO
}

type Buttons struct {
	events   chan core.Button
	cancel   context.CancelFunc
	bindings []buttonBinding
	wg       sync.WaitGroup
	once     sync.Once
}

func NewButtons(debounce time.Duration) (*Buttons, error) {
	if debounce <= 0 {
		return nil, fmt.Errorf("button debounce must be positive")
	}
	ctx, cancel := context.WithCancel(context.Background())
	buttons := &Buttons{
		events: make(chan core.Button, 16),
		cancel: cancel,
	}
	definitions := []struct {
		pinName string
		button  core.Button
	}{
		{pinName: "GPIO21", button: core.ButtonPrevious},
		{pinName: "GPIO20", button: core.ButtonNext},
		{pinName: "GPIO16", button: core.ButtonReserved},
	}
	for _, definition := range definitions {
		pin := gpioreg.ByName(definition.pinName)
		if pin == nil {
			cancel()
			_ = buttons.haltPins()
			return nil, fmt.Errorf("button pin %s is unavailable", definition.pinName)
		}
		if err := pin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
			cancel()
			_ = buttons.haltPins()
			return nil, fmt.Errorf("configure button pin %s: %w", definition.pinName, err)
		}
		buttons.bindings = append(buttons.bindings, buttonBinding{name: definition.pinName, button: definition.button, pin: pin})
	}
	for _, binding := range buttons.bindings {
		buttons.wg.Add(1)
		go buttons.watch(ctx, binding, debounce)
	}
	return buttons, nil
}

func (b *Buttons) Events() <-chan core.Button {
	return b.events
}

func (b *Buttons) Close() (err error) {
	b.once.Do(func() {
		b.cancel()
		b.wg.Wait()
		err = b.haltPins()
		close(b.events)
	})
	return err
}

func (b *Buttons) watch(ctx context.Context, binding buttonBinding, debounce time.Duration) {
	defer b.wg.Done()
	var lastAccepted time.Time
	for ctx.Err() == nil {
		if !binding.pin.WaitForEdge(100 * time.Millisecond) {
			continue
		}
		now := time.Now()
		if !acceptPress(&lastAccepted, now, debounce) {
			continue
		}
		select {
		case b.events <- binding.button:
		case <-ctx.Done():
			return
		}
	}
}

func acceptPress(lastAccepted *time.Time, now time.Time, debounce time.Duration) bool {
	if !lastAccepted.IsZero() && now.Sub(*lastAccepted) < debounce {
		return false
	}
	*lastAccepted = now
	return true
}

func (b *Buttons) haltPins() error {
	var errs []error
	for _, binding := range b.bindings {
		if err := binding.pin.Halt(); err != nil {
			errs = append(errs, fmt.Errorf("halt %s: %w", binding.name, err))
		}
	}
	return errors.Join(errs...)
}
