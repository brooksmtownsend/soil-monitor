package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"soilmonitor/internal/core"
)

type fakeDisplay struct {
	mu     sync.Mutex
	views  []core.View
	closed bool
}

func (f *fakeDisplay) Render(view core.View) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.views = append(f.views, view)
	return nil
}

func (f *fakeDisplay) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeDisplay) snapshot() []core.View {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.View(nil), f.views...)
}

type fakeButtons struct {
	events chan core.Button
	closed atomic.Bool
}

func (f *fakeButtons) Events() <-chan core.Button { return f.events }
func (f *fakeButtons) Close() error {
	f.closed.Store(true)
	return nil
}

type fakeMQTT struct {
	events  chan core.MQTTEvent
	started atomic.Bool
	closed  atomic.Bool
}

type fakeWeb struct {
	started atomic.Bool
	stopped atomic.Bool
	runErr  error
}

func (f *fakeWeb) Run(ctx context.Context) error {
	f.started.Store(true)
	if f.runErr != nil {
		return f.runErr
	}
	<-ctx.Done()
	f.stopped.Store(true)
	return nil
}

func (f *fakeMQTT) Start(context.Context) error {
	f.started.Store(true)
	return nil
}
func (f *fakeMQTT) Events() <-chan core.MQTTEvent { return f.events }
func (f *fakeMQTT) Close() error {
	f.closed.Store(true)
	return nil
}

func TestRunnerIntegration(t *testing.T) {
	state, err := core.NewState([]core.PlantDefinition{
		{ID: "money-tree", Name: "Money Tree"},
		{ID: "pothos", Name: "Pothos"},
		{ID: "snake-plant", Name: "Snake Plant"},
	})
	if err != nil {
		t.Fatal(err)
	}
	display := &fakeDisplay{}
	buttons := &fakeButtons{events: make(chan core.Button, 8)}
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent, 8)}
	web := &fakeWeb{}
	runner := Runner{
		State: state, Display: display, Buttons: buttons, MQTT: mqtt, Web: web,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	waitForViews(t, display, 1)
	if !mqtt.started.Load() {
		t.Fatal("MQTT adapter was not started")
	}
	waitForCondition(t, func() bool { return web.started.Load() }, "web adapter start")

	mqtt.events <- core.MQTTEvent{Kind: core.MQTTConnected}
	waitForViews(t, display, 2)
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/money-tree/moisture", Payload: []byte("47")}
	waitForViews(t, display, 3)
	selected := display.snapshot()[2]
	if selected.Moisture == nil || *selected.Moisture != 47 {
		t.Fatalf("selected update was not rendered: %#v", selected)
	}
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/pothos/moisture", Payload: []byte("72")}
	time.Sleep(20 * time.Millisecond)
	if got := len(display.snapshot()); got != 3 {
		t.Fatalf("non-selected update caused render; views = %d", got)
	}

	buttons.events <- core.ButtonNext
	waitForViews(t, display, 4)
	view := display.snapshot()[3]
	if view.PlantID != "pothos" || view.Moisture == nil || *view.Moisture != 72 || !view.Connected {
		t.Fatalf("unexpected next-plant view: %#v", view)
	}

	mqtt.events <- core.MQTTEvent{Kind: core.MQTTDisconnected}
	waitForViews(t, display, 5)
	view = display.snapshot()[4]
	if view.Connected || view.Moisture == nil || *view.Moisture != 72 {
		t.Fatalf("disconnect did not preserve reading: %#v", view)
	}
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTConnected}
	waitForViews(t, display, 6)
	view = display.snapshot()[5]
	if !view.Connected || view.Moisture == nil || *view.Moisture != 72 {
		t.Fatalf("reconnect did not preserve reading: %#v", view)
	}
	buttons.events <- core.ButtonReserved
	time.Sleep(20 * time.Millisecond)
	if got := len(display.snapshot()); got != 6 {
		t.Fatalf("reserved button caused render; views = %d", got)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
	if !mqtt.closed.Load() || !buttons.closed.Load() || !display.closed {
		t.Fatalf("resources not closed: mqtt=%v buttons=%v display=%v", mqtt.closed.Load(), buttons.closed.Load(), display.closed)
	}
	if !web.stopped.Load() {
		t.Fatal("web adapter was not stopped")
	}
}

func TestRunnerReturnsUnexpectedWebFailure(t *testing.T) {
	state, err := core.NewState([]core.PlantDefinition{
		{ID: "money-tree", Name: "Money Tree"},
		{ID: "pothos", Name: "Pothos"},
		{ID: "snake-plant", Name: "Snake Plant"},
	})
	if err != nil {
		t.Fatal(err)
	}
	display := &fakeDisplay{}
	buttons := &fakeButtons{events: make(chan core.Button)}
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent)}
	web := &fakeWeb{runErr: errors.New("listen failed")}
	runner := Runner{
		State: state, Display: display, Buttons: buttons, MQTT: mqtt, Web: web,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	err = runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "web server: listen failed") {
		t.Fatalf("Run() error = %v", err)
	}
	if !mqtt.closed.Load() || !buttons.closed.Load() || !display.closed {
		t.Fatalf("resources not closed after web failure: mqtt=%v buttons=%v display=%v", mqtt.closed.Load(), buttons.closed.Load(), display.closed)
	}
}

func waitForViews(t *testing.T, display *fakeDisplay, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(display.snapshot()) >= count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d rendered views", count)
}

func waitForCondition(t *testing.T, condition func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}
