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
	"soilmonitor/internal/history"
)

type fakeDisplay struct {
	mu     sync.Mutex
	views  []core.View
	closed atomic.Bool
}

func (f *fakeDisplay) Render(view core.View) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.views = append(f.views, view)
	return nil
}
func (f *fakeDisplay) Close() error { f.closed.Store(true); return nil }
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
func (f *fakeButtons) Close() error               { f.closed.Store(true); return nil }

type fakeMQTT struct {
	events          chan core.MQTTEvent
	started, closed atomic.Bool
}

func (f *fakeMQTT) Start(context.Context) error   { f.started.Store(true); return nil }
func (f *fakeMQTT) Events() <-chan core.MQTTEvent { return f.events }
func (f *fakeMQTT) Close() error                  { f.closed.Store(true); return nil }

type fakeWeb struct {
	started, stopped atomic.Bool
	runErr           error
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

type fakeHistory struct {
	mu        sync.Mutex
	readings  []history.Reading
	names     map[string]string
	appendErr error
	renameErr error
	closed    atomic.Bool
}

func (f *fakeHistory) Plants(context.Context) ([]history.Plant, error) { return nil, nil }
func (f *fakeHistory) AppendReading(_ context.Context, reading history.Reading) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendErr != nil {
		return f.appendErr
	}
	f.readings = append(f.readings, reading)
	return nil
}
func (f *fakeHistory) RenamePlant(_ context.Context, id, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.renameErr != nil {
		return f.renameErr
	}
	if f.names == nil {
		f.names = map[string]string{}
	}
	f.names[id] = name
	return nil
}
func (f *fakeHistory) Readings(context.Context, string, int) ([]history.Reading, error) {
	return nil, nil
}
func (f *fakeHistory) Close() error { f.closed.Store(true); return nil }
func (f *fakeHistory) count() int   { f.mu.Lock(); defer f.mu.Unlock(); return len(f.readings) }
func (f *fakeHistory) failAppends(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.appendErr = err
}

func TestRunnerDiscoversPersistsNavigatesAndRenames(t *testing.T) {
	state, _ := core.NewState(nil)
	display := &fakeDisplay{}
	buttons := &fakeButtons{events: make(chan core.Button, 8)}
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent, 8)}
	web := &fakeWeb{}
	repository := &fakeHistory{}
	commands := NewCommander()
	now := time.Date(2026, 8, 24, 16, 0, 0, 0, time.UTC)
	runner := Runner{State: state, Display: display, Buttons: buttons, MQTT: mqtt, Web: web, History: repository, Commands: commands, Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	waitForViews(t, display, 1)
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTConnected}
	waitForViews(t, display, 2)
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/money-tree/moisture", Payload: []byte(`{"raw":32000,"percentage":47}`)}
	waitForViews(t, display, 3)
	if repository.count() != 1 || state.View().Latest == nil || state.View().Latest.Percentage != 47 {
		t.Fatalf("reading not persisted/projected: count=%d view=%#v", repository.count(), state.View())
	}
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/pothos/moisture", Payload: []byte(`{"raw":33000,"percentage":72}`)}
	waitForCondition(t, func() bool { return repository.count() == 2 }, "second persisted reading")
	if len(display.snapshot()) != 3 {
		t.Fatal("non-selected plant caused redraw")
	}
	buttons.events <- core.ButtonNext
	waitForViews(t, display, 4)
	if state.View().PlantID != "pothos" {
		t.Fatalf("next view = %#v", state.View())
	}
	if err := commands.RenamePlant(ctx, "pothos", "  Kitchen Pothos  "); err != nil {
		t.Fatal(err)
	}
	waitForViews(t, display, 5)
	if state.View().PlantName != "Kitchen Pothos" {
		t.Fatalf("renamed view = %#v", state.View())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !mqtt.closed.Load() || !buttons.closed.Load() || !display.closed.Load() || !web.stopped.Load() || !repository.closed.Load() {
		t.Fatal("adapters were not closed")
	}
}

func TestRunnerRejectsInvalidAndPreservesStateOnStorageFailure(t *testing.T) {
	state, _ := core.NewState(nil)
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent, 4)}
	repository := &fakeHistory{}
	runner := Runner{State: state, MQTT: mqtt, History: repository, Now: func() time.Time { return time.Now().UTC() }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	waitForCondition(t, func() bool { return mqtt.started.Load() }, "MQTT start")
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/legacy/moisture", Payload: []byte("47")}
	time.Sleep(10 * time.Millisecond)
	if repository.count() != 0 || len(state.Snapshot().Plants) != 0 {
		t.Fatal("invalid message changed state")
	}
	repository.failAppends(errors.New("disk full"))
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/aloe/moisture", Payload: []byte(`{"raw":1,"percentage":2}`)}
	time.Sleep(10 * time.Millisecond)
	if len(state.Snapshot().Plants) != 0 {
		t.Fatal("failed persistence changed state")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCommanderValidationAndStorageFailure(t *testing.T) {
	state, _ := core.NewState(nil)
	state.ApplyReading(core.Reading{PlantID: "aloe", Raw: 1, Percentage: 2, ObservedAt: time.Now()})
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent)}
	repository := &fakeHistory{renameErr: history.ErrPlantNotFound}
	commands := NewCommander()
	runner := Runner{State: state, MQTT: mqtt, History: repository, Commands: commands}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	waitForCondition(t, func() bool { return mqtt.started.Load() }, "MQTT start")
	if err := commands.RenamePlant(ctx, "aloe", " "); err == nil {
		t.Fatal("expected invalid name error")
	}
	if err := commands.RenamePlant(ctx, "aloe", "Aloe"); !errors.Is(err, history.ErrPlantNotFound) {
		t.Fatalf("rename error = %v", err)
	}
	if state.View().PlantName != "aloe" {
		t.Fatal("failed rename changed state")
	}
	cancel()
	<-done
}

func TestRunnerReturnsUnexpectedWebFailure(t *testing.T) {
	state, _ := core.NewState(nil)
	display := &fakeDisplay{}
	buttons := &fakeButtons{events: make(chan core.Button)}
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent)}
	err := (&Runner{State: state, Display: display, Buttons: buttons, MQTT: mqtt, Web: &fakeWeb{runErr: errors.New("listen failed")}, History: &fakeHistory{}}).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "web server: listen failed") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRejectsMissingHistoryAndPartialHardware(t *testing.T) {
	state, _ := core.NewState(nil)
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent)}
	if err := (&Runner{State: state, MQTT: mqtt}).Run(context.Background()); err == nil || !strings.Contains(err.Error(), "history") {
		t.Fatalf("error = %v", err)
	}
	if err := (&Runner{State: state, MQTT: mqtt, History: &fakeHistory{}, Display: &fakeDisplay{}}).Run(context.Background()); err == nil || !strings.Contains(err.Error(), "display and buttons") {
		t.Fatalf("error = %v", err)
	}
}

func waitForViews(t *testing.T, display *fakeDisplay, count int) {
	t.Helper()
	waitForCondition(t, func() bool { return len(display.snapshot()) >= count }, "rendered views")
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
