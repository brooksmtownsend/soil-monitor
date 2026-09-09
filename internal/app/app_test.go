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
	notifications    atomic.Int64
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

func (f *fakeWeb) Notify(core.Snapshot) { f.notifications.Add(1) }

type fakeHistory struct {
	mu          sync.Mutex
	readings    []history.Reading
	names       map[string]string
	plants      map[string]history.Plant
	photos      map[string]history.Photo
	appendErr   error
	renameErr   error
	registerErr error
	scheduleErr error
	wateringErr error
	photoErr    error
	closed      atomic.Bool
}

func (f *fakeHistory) Plants(context.Context) ([]history.Plant, error) { return nil, nil }
func (f *fakeHistory) AppendReading(_ context.Context, reading history.Reading) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendErr != nil {
		return f.appendErr
	}
	f.readings = append(f.readings, reading)
	if f.plants != nil {
		plant := f.plants[reading.PlantID]
		plant.ID = reading.PlantID
		plant.LastSeen = reading.ObservedAt
		if plant.FirstSeen.IsZero() {
			plant.FirstSeen = reading.ObservedAt
		}
		plant.Latest = &reading
		f.plants[reading.PlantID] = plant
	}
	return nil
}
func (f *fakeHistory) RegisterPlant(_ context.Context, id, name string, cadence core.Cadence, at time.Time, photo *history.Photo) (history.Plant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.registerErr != nil {
		return history.Plant{}, f.registerErr
	}
	if f.plants == nil {
		f.plants = map[string]history.Plant{}
	}
	if _, exists := f.plants[id]; exists {
		return history.Plant{}, history.ErrPlantConflict
	}
	next, _ := core.NextWateringDate(cadence, at)
	plant := history.Plant{ID: id, Name: name, FirstSeen: at.UTC(), LastSeen: at.UTC(), Cadence: cadence, NextWateringDate: next, HasPhoto: photo != nil}
	f.plants[id] = plant
	if photo != nil {
		if f.photos == nil {
			f.photos = map[string]history.Photo{}
		}
		f.photos[id] = *photo
	}
	return plant, nil
}
func (f *fakeHistory) SetSchedule(_ context.Context, id string, cadence core.Cadence, at time.Time) (history.Plant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.scheduleErr != nil {
		return history.Plant{}, f.scheduleErr
	}
	plant, ok := f.plants[id]
	if !ok {
		return history.Plant{}, history.ErrPlantNotFound
	}
	plant.Cadence = cadence
	plant.NextWateringDate, _ = core.NextWateringDate(cadence, at)
	f.plants[id] = plant
	return plant, nil
}
func (f *fakeHistory) ConfirmWatering(_ context.Context, id string, at time.Time) (history.Plant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.wateringErr != nil {
		return history.Plant{}, f.wateringErr
	}
	plant, ok := f.plants[id]
	if !ok {
		return history.Plant{}, history.ErrPlantNotFound
	}
	if plant.Cadence != "" {
		plant.NextWateringDate, _ = core.NextWateringDate(plant.Cadence, at)
	}
	f.plants[id] = plant
	return plant, nil
}
func (f *fakeHistory) ReplacePhoto(_ context.Context, id string, photo history.Photo) (history.Plant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.photoErr != nil {
		return history.Plant{}, f.photoErr
	}
	plant, ok := f.plants[id]
	if !ok {
		return history.Plant{}, history.ErrPlantNotFound
	}
	plant.HasPhoto = true
	f.plants[id] = plant
	if f.photos == nil {
		f.photos = map[string]history.Photo{}
	}
	f.photos[id] = photo
	return plant, nil
}
func (f *fakeHistory) Photo(_ context.Context, id string) (history.Photo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	photo, ok := f.photos[id]
	if !ok {
		return history.Photo{}, history.ErrPhotoNotFound
	}
	return photo, nil
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
func (f *fakeHistory) Readings(context.Context, string, time.Time, time.Time, int) ([]history.Reading, error) {
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

func TestRunnerCoordinatesRegistrationScheduleWateringAndPhoto(t *testing.T) {
	state, _ := core.NewState(nil)
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent, 4)}
	repository := &fakeHistory{}
	commands := NewCommander()
	web := &fakeWeb{}
	var nowUnix atomic.Int64
	nowUnix.Store(time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC).Unix())
	runner := Runner{State: state, MQTT: mqtt, Web: web, History: repository, Commands: commands, Now: func() time.Time { return time.Unix(nowUnix.Load(), 0).UTC() }}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	waitForCondition(t, func() bool { return mqtt.started.Load() }, "MQTT start")
	jpeg := &history.Photo{Bytes: []byte{0xff, 0xd8, 0xff}, MediaType: "image/jpeg"}
	id, err := commands.RegisterPlant(ctx, "  Money Tree  ", core.CadenceWeekly, jpeg)
	if err != nil || id != "money-tree" {
		t.Fatalf("registration = %q, %v", id, err)
	}
	plant, ok := state.Plant(id)
	if !ok || plant.Name != "Money Tree" || plant.NextWateringDate != "2026-09-15" || !plant.HasPhoto || plant.Latest != nil {
		t.Fatalf("registered state = %#v, %v", plant, ok)
	}
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/money-tree/moisture", Payload: []byte(`{"raw":32000,"percentage":47}`)}
	waitForCondition(t, func() bool { current, _ := state.Plant(id); return current.Latest != nil }, "registered plant reading")
	plant, _ = state.Plant(id)
	if plant.Cadence != core.CadenceWeekly || !plant.HasPhoto {
		t.Fatalf("MQTT update lost care metadata: %#v", plant)
	}
	nowUnix.Store(time.Date(2027, 1, 31, 17, 0, 0, 0, time.UTC).Unix())
	if err := commands.SetSchedule(ctx, id, core.CadenceMonthly); err != nil {
		t.Fatal(err)
	}
	plant, _ = state.Plant(id)
	if plant.NextWateringDate != "2027-02-28" {
		t.Fatalf("scheduled state = %#v", plant)
	}
	nowUnix.Store(time.Date(2027, 3, 3, 17, 0, 0, 0, time.UTC).Unix())
	if err := commands.ConfirmWatering(ctx, id); err != nil {
		t.Fatal(err)
	}
	plant, _ = state.Plant(id)
	if plant.NextWateringDate != "2027-04-03" {
		t.Fatalf("watered state = %#v", plant)
	}
	png := history.Photo{Bytes: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, MediaType: "image/png"}
	if err := commands.ReplacePhoto(ctx, id, png); err != nil {
		t.Fatal(err)
	}
	if web.notifications.Load() < 4 {
		t.Fatalf("notifications = %d", web.notifications.Load())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCareCommandValidationCancellationAndPersistenceFailures(t *testing.T) {
	var unavailable *Commander
	if err := unavailable.ConfirmWatering(context.Background(), "aloe"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unavailable commander error = %v", err)
	}
	commands := NewCommander()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := commands.RegisterPlant(canceled, "Aloe", core.CadenceWeekly, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled registration error = %v", err)
	}
	if _, err := commands.RegisterPlant(context.Background(), "界界", core.CadenceWeekly, nil); err == nil {
		t.Fatal("expected invalid derived identifier")
	}
	if _, err := commands.RegisterPlant(context.Background(), "Aloe", "daily", nil); err == nil {
		t.Fatal("expected invalid cadence")
	}
	if err := commands.ReplacePhoto(context.Background(), "aloe", history.Photo{Bytes: []byte("bad"), MediaType: "image/jpeg"}); err == nil {
		t.Fatal("expected invalid photo")
	}

	state, _ := core.NewState(nil)
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent)}
	repository := &fakeHistory{registerErr: errors.New("disk full")}
	running := NewCommander()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (&Runner{State: state, MQTT: mqtt, History: repository, Commands: running}).Run(ctx) }()
	waitForCondition(t, func() bool { return mqtt.started.Load() }, "MQTT start")
	if _, err := running.RegisterPlant(ctx, "Aloe", core.CadenceWeekly, nil); err == nil || len(state.Snapshot().Plants) != 0 {
		t.Fatalf("failed registration error = %v, state = %#v", err, state.Snapshot())
	}
	repository.registerErr = nil
	if _, err := running.RegisterPlant(ctx, "Aloe", core.CadenceWeekly, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := running.RegisterPlant(ctx, "Aloe", core.CadenceWeekly, nil); !errors.Is(err, history.ErrPlantConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	repository.scheduleErr = errors.New("disk full")
	before, _ := state.Plant("aloe")
	if err := running.SetSchedule(ctx, "aloe", core.CadenceMonthly); err == nil {
		t.Fatal("expected schedule failure")
	}
	after, _ := state.Plant("aloe")
	if after.NextWateringDate != before.NextWateringDate {
		t.Fatalf("failed schedule changed state: before=%#v after=%#v", before, after)
	}
	stop()
	<-done
}

func TestRunnerSerializesRegistrationAlongsideMQTT(t *testing.T) {
	state, _ := core.NewState(nil)
	mqtt := &fakeMQTT{events: make(chan core.MQTTEvent, 1)}
	repository := &fakeHistory{}
	commands := NewCommander()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	commandTime := time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC)
	go func() {
		done <- (&Runner{State: state, MQTT: mqtt, History: repository, Commands: commands, Now: func() time.Time { return commandTime }}).Run(ctx)
	}()
	waitForCondition(t, func() bool { return mqtt.started.Load() }, "MQTT start")
	registered := make(chan error, 1)
	go func() {
		_, err := commands.RegisterPlant(ctx, "Aloe", core.CadenceWeekly, nil)
		registered <- err
	}()
	mqtt.events <- core.MQTTEvent{Kind: core.MQTTMessage, Topic: "plants/pothos/moisture", Payload: []byte(`{"raw":10,"percentage":20}`)}
	if err := <-registered; err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, func() bool { return len(state.Snapshot().Plants) == 2 }, "both serialized updates")
	if aloe, ok := state.Plant("aloe"); !ok || aloe.Cadence != core.CadenceWeekly {
		t.Fatalf("registered plant = %#v, %v", aloe, ok)
	}
	if pothos, ok := state.Plant("pothos"); !ok || pothos.Latest == nil || pothos.Latest.Percentage != 20 {
		t.Fatalf("MQTT plant = %#v, %v", pothos, ok)
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
