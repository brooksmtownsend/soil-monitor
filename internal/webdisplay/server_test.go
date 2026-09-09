package webdisplay

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"soilmonitor/internal/core"
	"soilmonitor/internal/history"
)

type blockingListener struct {
	closed chan struct{}
	once   sync.Once
}

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}
func (l *blockingListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *blockingListener) Addr() net.Addr { return testAddr("test") }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

type testCommands struct {
	state *core.State
	store *history.Store
	now   time.Time
	err   error
}

type recordingHistory struct {
	calls    int
	plantID  string
	start    time.Time
	end      time.Time
	limit    int
	readings []history.Reading
}

func (h *recordingHistory) Readings(_ context.Context, plantID string, start, end time.Time, limit int) ([]history.Reading, error) {
	h.calls++
	h.plantID = plantID
	h.start = start
	h.end = end
	h.limit = limit
	return h.readings, nil
}

func (h *recordingHistory) Photo(context.Context, string) (history.Photo, error) {
	return history.Photo{}, history.ErrPhotoNotFound
}

func (r *testCommands) RenamePlant(ctx context.Context, id, name string) error {
	if r.err != nil {
		return r.err
	}
	if err := r.store.RenamePlant(ctx, id, name); err != nil {
		return err
	}
	r.state.RenamePlant(id, name)
	return nil
}

func (r *testCommands) RegisterPlant(ctx context.Context, name string, cadence core.Cadence, photo *history.Photo) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	id, displayName, err := core.DerivePlantID(name)
	if err != nil {
		return "", err
	}
	plant, err := r.store.RegisterPlant(ctx, id, displayName, cadence, r.commandTime(), photo)
	if err != nil {
		return "", err
	}
	r.state.ApplyPlant(webCorePlant(plant))
	return id, nil
}

func (r *testCommands) SetSchedule(ctx context.Context, id string, cadence core.Cadence) error {
	if r.err != nil {
		return r.err
	}
	plant, err := r.store.SetSchedule(ctx, id, cadence, r.commandTime())
	if err == nil {
		r.state.ApplyPlant(webCorePlant(plant))
	}
	return err
}

func (r *testCommands) ConfirmWatering(ctx context.Context, id string) error {
	if r.err != nil {
		return r.err
	}
	plant, err := r.store.ConfirmWatering(ctx, id, r.commandTime())
	if err == nil {
		r.state.ApplyPlant(webCorePlant(plant))
	}
	return err
}

func (r *testCommands) ReplacePhoto(ctx context.Context, id string, photo history.Photo) error {
	if r.err != nil {
		return r.err
	}
	plant, err := r.store.ReplacePhoto(ctx, id, photo)
	if err == nil {
		r.state.ApplyPlant(webCorePlant(plant))
	}
	return err
}

var testWebNow = time.Date(2026, 9, 8, 16, 0, 0, 0, time.UTC)

func (r *testCommands) commandTime() time.Time {
	if r.now.IsZero() {
		return testWebNow
	}
	return r.now
}

func webCorePlant(plant history.Plant) core.Plant {
	converted := core.Plant{ID: plant.ID, Name: plant.Name, FirstSeen: plant.FirstSeen, LastSeen: plant.LastSeen, Cadence: plant.Cadence, NextWateringDate: plant.NextWateringDate, HasPhoto: plant.HasPhoto}
	if plant.Latest != nil {
		converted.Latest = &core.Reading{PlantID: plant.ID, Raw: plant.Latest.Raw, Percentage: plant.Latest.Percentage, ObservedAt: plant.Latest.ObservedAt}
	}
	return converted
}

func testServer(t *testing.T, readingCount int) (*Server, *core.State, *history.Store) {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	state, _ := core.NewState(nil)
	start := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	for i := 0; i < readingCount; i++ {
		reading := core.Reading{PlantID: "money-tree", Raw: 30000 + i, Percentage: i % 101, ObservedAt: start.Add(time.Duration(i) * time.Minute)}
		if err := store.AppendReading(context.Background(), history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt}); err != nil {
			t.Fatal(err)
		}
		state.ApplyReading(reading)
	}
	server, err := New(":0", state, store, &testCommands{state: state, store: store})
	if err != nil {
		t.Fatal(err)
	}
	server.now = func() time.Time { return start.Add(time.Duration(readingCount) * time.Minute) }
	return server, state, store
}

func TestHistoryRangeStarts(t *testing.T) {
	end := time.Date(2028, 3, 31, 14, 15, 16, 17, time.FixedZone("offset", -4*60*60))
	tests := []struct {
		key  string
		want time.Time
	}{
		{key: "1d", want: time.Date(2028, 3, 30, 18, 15, 16, 17, time.UTC)},
		{key: "1w", want: time.Date(2028, 3, 24, 18, 15, 16, 17, time.UTC)},
		{key: "1mo", want: time.Date(2028, 2, 29, 18, 15, 16, 17, time.UTC)},
		{key: "3mo", want: time.Date(2027, 12, 31, 18, 15, 16, 17, time.UTC)},
		{key: "6mo", want: time.Date(2027, 9, 30, 18, 15, 16, 17, time.UTC)},
		{key: "1y", want: time.Date(2027, 3, 31, 18, 15, 16, 17, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/plants/money-tree?range="+tt.key, nil)
			preset, err := parseHistoryRange(request)
			if err != nil {
				t.Fatal(err)
			}
			if got := preset.start(end.UTC()); !got.Equal(tt.want) || got.Location() != time.UTC {
				t.Fatalf("start = %v, want %v", got, tt.want)
			}
		})
	}
	if got := subtractCalendarMonths(time.Date(2028, 2, 29, 10, 0, 0, 0, time.UTC), 12); !got.Equal(time.Date(2027, 2, 28, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("leap-year start = %v", got)
	}
}

func TestEasternTimeFormattingTracksDaylightSavingTime(t *testing.T) {
	tests := []struct {
		name     string
		observed time.Time
		want     string
	}{
		{name: "summer EDT", observed: time.Date(2026, 7, 1, 16, 30, 0, 0, time.UTC), want: "2026-07-01 12:30:00 EDT"},
		{name: "winter EST", observed: time.Date(2026, 1, 1, 17, 30, 0, 0, time.UTC), want: "2026-01-01 12:30:00 EST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatEasternTime(tt.observed); got != tt.want {
				t.Fatalf("formatted time = %q, want %q", got, tt.want)
			}
			machine, err := time.Parse(time.RFC3339, formatMachineTime(tt.observed))
			if err != nil {
				t.Fatalf("parse machine time: %v", err)
			}
			if !machine.Equal(tt.observed) {
				t.Fatalf("machine time = %v, want instant %v", machine, tt.observed)
			}
		})
	}
}

func TestDetailRangeRequests(t *testing.T) {
	now := time.Date(2028, 3, 31, 18, 15, 16, 0, time.UTC)
	state, _ := core.NewState(nil)
	state.ApplyReading(core.Reading{PlantID: "money-tree", Raw: 1, Percentage: 2, ObservedAt: now})
	historySource := &recordingHistory{}
	server, err := New(":0", state, historySource, &testCommands{})
	if err != nil {
		t.Fatal(err)
	}
	server.now = func() time.Time { return now }

	tests := []struct {
		path      string
		wantStart time.Time
	}{
		{path: "/plants/money-tree", wantStart: now.Add(-24 * time.Hour)},
		{path: "/plants/money-tree?range=1d", wantStart: now.Add(-24 * time.Hour)},
		{path: "/plants/money-tree?range=1w", wantStart: now.Add(-7 * 24 * time.Hour)},
		{path: "/plants/money-tree?range=1mo", wantStart: time.Date(2028, 2, 29, 18, 15, 16, 0, time.UTC)},
		{path: "/plants/money-tree?range=3mo", wantStart: time.Date(2027, 12, 31, 18, 15, 16, 0, time.UTC)},
		{path: "/plants/money-tree?range=6mo", wantStart: time.Date(2027, 9, 30, 18, 15, 16, 0, time.UTC)},
		{path: "/plants/money-tree?range=1y", wantStart: time.Date(2027, 3, 31, 18, 15, 16, 0, time.UTC)},
	}
	for _, tt := range tests {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if response.Code != http.StatusOK || !historySource.start.Equal(tt.wantStart) || !historySource.end.Equal(now) || historySource.limit != historyLimit {
			t.Errorf("%s: status=%d start=%v end=%v limit=%d", tt.path, response.Code, historySource.start, historySource.end, historySource.limit)
		}
	}

	calls := historySource.calls
	for _, path := range []string{"/plants/money-tree?range=", "/plants/money-tree?range=all", "/plants/money-tree?range=1d&range=1w"} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status=%d", path, response.Code)
		}
	}
	if historySource.calls != calls {
		t.Fatal("invalid range queried history")
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/unknown?range=1d", nil))
	if response.Code != http.StatusNotFound || historySource.calls != calls {
		t.Fatalf("unknown plant: status=%d calls=%d", response.Code, historySource.calls)
	}
}

func TestOverviewEmptyAndPopulated(t *testing.T) {
	server, state, store := testServer(t, 0)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "No plants yet") || !strings.Contains(response.Body.String(), "/plants/new") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("empty response: %d %s", response.Code, response.Body.String())
	}
	reading := core.Reading{PlantID: "money-tree", Raw: 32768, Percentage: 47, ObservedAt: time.Date(2026, 7, 1, 16, 30, 0, 0, time.UTC)}
	if err := store.AppendReading(context.Background(), history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt}); err != nil {
		t.Fatal(err)
	}
	state.ApplyReading(reading)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, text := range []string{"money-tree", "47%", "Raw: 32768", `/plants/money-tree`, `datetime="2026-07-01T16:30:00Z"`, "2026-07-01 12:30:00 EDT"} {
		if !strings.Contains(body, text) {
			t.Fatalf("overview missing %q: %s", text, body)
		}
	}
}

func TestRegistrationRoutesAndMultipartValidation(t *testing.T) {
	server, state, store := testServer(t, 0)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/new", nil))
	for _, token := range []string{"Add plant", `enctype="multipart/form-data"`, "Weekly", "Biweekly", "Monthly", "As needed", "20 MiB", "HEIC"} {
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), token) {
			t.Fatalf("registration form missing %q: %d %s", token, response.Code, response.Body.String())
		}
	}

	request := multipartRequest(t, "/plants", "  Monstéra Deliciosa  ", "weekly", "", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/plants/monstera-deliciosa" {
		t.Fatalf("registration response = %d %q %s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	plant, ok := state.Plant("monstera-deliciosa")
	if !ok || plant.Name != "Monstéra Deliciosa" || plant.NextWateringDate != "2026-09-15" || plant.HasPhoto {
		t.Fatalf("registered plant = %#v, %v", plant, ok)
	}

	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0}
	request = multipartRequest(t, "/plants", "Aloe", "as-needed", "iphone.jpg", jpeg)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("photo registration = %d %s", response.Code, response.Body.String())
	}
	if photo, err := store.Photo(context.Background(), "aloe"); err != nil || !bytes.Equal(photo.Bytes, jpeg) || photo.MediaType != "image/jpeg" {
		t.Fatalf("registered photo = %#v, %v", photo, err)
	}

	for _, tt := range []struct {
		name, cadence, filename string
		photo                   []byte
		want                    int
	}{
		{name: "Aloe", cadence: "weekly", want: http.StatusConflict},
		{name: " ", cadence: "weekly", want: http.StatusBadRequest},
		{name: "Fern", cadence: "daily", want: http.StatusBadRequest},
		{name: "Fern", cadence: "weekly", filename: "empty.jpg", photo: []byte{}, want: http.StatusUnsupportedMediaType},
		{name: "Fern", cadence: "weekly", filename: "fake.jpg", photo: []byte("not an image"), want: http.StatusUnsupportedMediaType},
		{name: "Fern", cadence: "weekly", filename: "large.jpg", photo: oversizedJPEG(), want: http.StatusRequestEntityTooLarge},
		{name: "Fern", cadence: "weekly", filename: "massive.jpg", photo: massiveJPEG(), want: http.StatusRequestEntityTooLarge},
	} {
		request = multipartRequest(t, "/plants", tt.name, tt.cadence, tt.filename, tt.photo)
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != tt.want {
			t.Errorf("registration %q/%q = %d, want %d: %s", tt.name, tt.cadence, response.Code, tt.want, response.Body.String())
		}
	}
	request = httptest.NewRequest(http.MethodPost, "/plants", strings.NewReader("broken"))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed multipart = %d", response.Code)
	}
	if cap(server.uploadSlots) != 1 {
		t.Fatalf("upload slot capacity = %d", cap(server.uploadSlots))
	}
}

func TestRegistrationPersistenceFailureAndMQTTConflict(t *testing.T) {
	server, state, store := testServer(t, 1)
	request := multipartRequest(t, "/plants", "Money Tree", "weekly", "", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("MQTT conflict = %d %s", response.Code, response.Body.String())
	}
	failing, err := New(":0", state, store, &testCommands{state: state, store: store, err: errors.New("disk full")})
	if err != nil {
		t.Fatal(err)
	}
	request = multipartRequest(t, "/plants", "Aloe", "weekly", "", nil)
	response = httptest.NewRecorder()
	failing.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("registration persistence failure = %d", response.Code)
	}
	if _, ok := state.Plant("aloe"); ok {
		t.Fatal("failed registration changed state")
	}
}

func TestPhotoUploadAcceptsExactLimitAndCleansTemporaryFile(t *testing.T) {
	server, _, store := testServer(t, 0)
	photo := make([]byte, core.MaxPhotoBytes)
	copy(photo, []byte{0xff, 0xd8, 0xff})
	request := multipartRequest(t, "/plants", "Large Photo Plant", "weekly", "large.jpg", photo)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("boundary upload = %d %s", response.Code, response.Body.String())
	}
	stored, err := store.Photo(context.Background(), "large-photo-plant")
	if err != nil || len(stored.Bytes) != core.MaxPhotoBytes {
		t.Fatalf("stored boundary photo bytes = %d, %v", len(stored.Bytes), err)
	}
	files := request.MultipartForm.File["photo"]
	if len(files) != 1 {
		t.Fatalf("multipart files = %d", len(files))
	}
	if reopened, err := files[0].Open(); err == nil {
		reopened.Close()
		t.Fatal("temporary multipart file remained after request")
	}
}

func TestUploadSlotHonorsCanceledRequest(t *testing.T) {
	server, _, _ := testServer(t, 0)
	server.uploadSlots <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := multipartRequest(t, "/plants", "Aloe", "weekly", "", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	<-server.uploadSlots
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("canceled queued upload = %d", response.Code)
	}
}

func TestScheduleWateringAndCarePresentation(t *testing.T) {
	server, state, _ := testServer(t, 1)
	postForm := func(path string, form url.Values) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}
	if response := postForm("/plants/money-tree/name", url.Values{"name": {"Money Tree"}}); response.Code != http.StatusSeeOther {
		t.Fatalf("rename response = %d", response.Code)
	}
	response := postForm("/plants/money-tree/schedule", url.Values{"cadence": {"monthly"}})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("schedule response = %d %s", response.Code, response.Body.String())
	}
	plant, _ := state.Plant("money-tree")
	if plant.NextWateringDate != "2026-10-08" {
		t.Fatalf("scheduled plant = %#v", plant)
	}
	response = postForm("/plants/money-tree/water", nil)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("watering response = %d", response.Code)
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	for _, token := range []string{"Money Tree", "Next watering:", "2026-10-08", "Confirm watered", "Save schedule", "Moisture history", "Upload photo"} {
		if !strings.Contains(response.Body.String(), token) {
			t.Errorf("scheduled detail missing %q", token)
		}
	}
	response = postForm("/plants/money-tree/schedule", url.Values{"cadence": {"as-needed"}})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("as-needed response = %d", response.Code)
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	if !strings.Contains(response.Body.String(), "Water as needed") {
		t.Fatal("as-needed state not rendered")
	}
	for _, tt := range []struct {
		path string
		form url.Values
		want int
	}{
		{path: "/plants/money-tree/schedule", form: url.Values{"cadence": {"daily"}}, want: 400},
		{path: "/plants/unknown/schedule", form: url.Values{"cadence": {"weekly"}}, want: 404},
		{path: "/plants/unknown/water", want: 404},
	} {
		if got := postForm(tt.path, tt.form).Code; got != tt.want {
			t.Errorf("%s = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestUnconfiguredPlantCarePresentation(t *testing.T) {
	server, _, _ := testServer(t, 1)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Watering is not scheduled") || !strings.Contains(response.Body.String(), "Confirm watered") {
		t.Fatalf("unconfigured detail = %d %s", response.Code, response.Body.String())
	}
}

func TestOverdueWateringAnchorsOnConfirmationDate(t *testing.T) {
	server, state, _ := testServer(t, 0)
	commands := server.commands.(*testCommands)
	request := multipartRequest(t, "/plants", "Aloe", "weekly", "", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatal(response.Body.String())
	}
	commands.now = time.Date(2026, 9, 18, 16, 0, 0, 0, time.UTC)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/plants/aloe/water", nil))
	plant, _ := state.Plant("aloe")
	if response.Code != http.StatusSeeOther || plant.NextWateringDate != "2026-09-25" {
		t.Fatalf("overdue watering = %d %#v", response.Code, plant)
	}
}

func TestCareMutationPersistenceFailures(t *testing.T) {
	_, state, store := testServer(t, 1)
	failing, err := New(":0", state, store, &testCommands{state: state, store: store, err: errors.New("disk full")})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		path    string
		request func() *http.Request
	}{
		{path: "/plants/money-tree/schedule", request: func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/plants/money-tree/schedule", strings.NewReader("cadence=weekly"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return r
		}},
		{path: "/plants/money-tree/water", request: func() *http.Request { return httptest.NewRequest(http.MethodPost, "/plants/money-tree/water", nil) }},
		{path: "/plants/money-tree/photo", request: func() *http.Request {
			return multipartRequest(t, "/plants/money-tree/photo", "", "", "photo.jpg", []byte{0xff, 0xd8, 0xff})
		}},
	} {
		response := httptest.NewRecorder()
		failing.Handler().ServeHTTP(response, tt.request())
		if response.Code != http.StatusInternalServerError {
			t.Errorf("%s persistence failure = %d", tt.path, response.Code)
		}
	}
}

func TestPhotoDeliveryReplacementAndOverview(t *testing.T) {
	server, state, store := testServer(t, 0)
	request := multipartRequest(t, "/plants", "Aloe", "weekly", "aloe.jpg", []byte{0xff, 0xd8, 0xff, 0xe0})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatal(response.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(method, "/plants/aloe/photo", nil))
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("Content-Length") != "4" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s photo response = %d %#v", method, response.Code, response.Header())
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD photo returned a body")
		}
	}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	request = multipartRequest(t, "/plants/aloe/photo", "", "", "aloe.png", png)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("photo replacement = %d %s", response.Code, response.Body.String())
	}
	photo, err := store.Photo(context.Background(), "aloe")
	if err != nil || photo.MediaType != "image/png" || !bytes.Equal(photo.Bytes, png) {
		t.Fatalf("replaced photo = %#v, %v", photo, err)
	}
	request = multipartRequest(t, "/plants/aloe/photo", "", "", "bad.png", []byte("bad"))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid replacement = %d", response.Code)
	}
	photo, _ = store.Photo(context.Background(), "aloe")
	if photo.MediaType != "image/png" {
		t.Fatal("invalid replacement discarded existing photo")
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Body.String(), `src="/plants/aloe/photo"`) || !strings.Contains(response.Body.String(), `alt="Photo of Aloe"`) || !strings.Contains(response.Body.String(), "Reading unknown") {
		t.Fatalf("photo overview = %s", response.Body.String())
	}
	state.ApplyPlant(core.Plant{ID: "fern", Name: "Fern", FirstSeen: testWebNow, LastSeen: testWebNow})
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/fern/photo", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing photo = %d", response.Code)
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/plants/aloe/photo", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("photo method = %d %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestPhotoRouteServesHEIC(t *testing.T) {
	server, _, _ := testServer(t, 0)
	heic := testHEICPhoto("mif1", "heic")
	request := multipartRequest(t, "/plants", "iPhone Plant", "weekly", "plant.heic", heic)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("HEIC registration = %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/iphone-plant/photo", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/heic" || !bytes.Equal(response.Body.Bytes(), heic) {
		t.Fatalf("HEIC photo = %d %q %x", response.Code, response.Header().Get("Content-Type"), response.Body.Bytes())
	}
}

func multipartRequest(t *testing.T, path, name, cadence, filename string, photo []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if name != "" {
		if err := writer.WriteField("name", name); err != nil {
			t.Fatal(err)
		}
	}
	if cadence != "" {
		if err := writer.WriteField("cadence", cadence); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		part, err := writer.CreateFormFile("photo", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(photo); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func oversizedJPEG() []byte {
	photo := make([]byte, core.MaxPhotoBytes+1)
	copy(photo, []byte{0xff, 0xd8, 0xff})
	return photo
}

func massiveJPEG() []byte {
	photo := make([]byte, multipartLimit+1)
	copy(photo, []byte{0xff, 0xd8, 0xff})
	return photo
}

func testHEICPhoto(major string, compatible ...string) []byte {
	size := 16 + len(compatible)*4
	photo := make([]byte, size)
	binary.BigEndian.PutUint32(photo[:4], uint32(size))
	copy(photo[4:8], "ftyp")
	copy(photo[8:12], major)
	for i, brand := range compatible {
		copy(photo[16+i*4:], brand)
	}
	return photo
}

func TestDetailDisplaysRangeAndReadingsInEasternTime(t *testing.T) {
	server, _, _ := testServer(t, 1)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	body := response.Body.String()
	for _, text := range []string{
		`datetime="2026-08-23T12:01:00Z">2026-08-23 08:01:00 EDT`,
		`datetime="2026-08-24T12:01:00Z">2026-08-24 08:01:00 EDT`,
		`datetime="2026-08-24T12:00:00Z">2026-08-24 08:00:00 EDT`,
	} {
		if !strings.Contains(body, text) {
			t.Fatalf("detail missing Eastern timestamp %q: %s", text, body)
		}
	}
}

func TestDetailChartIsBoundedAndChronological(t *testing.T) {
	server, _, _ := testServer(t, 1005)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "Moisture percentage history") || !strings.Contains(body, "Newest 1000 readings") {
		t.Fatalf("detail response: %d %s", response.Code, body)
	}
	if strings.Contains(body, ">30000<") || !strings.Contains(body, ">30005<") || !strings.Contains(body, ">31004<") {
		t.Fatal("history was not newest 1000")
	}
	if strings.Index(body, ">31004<") > strings.Index(body, ">30005<") {
		t.Fatal("history table is not newest first")
	}
	if !strings.Contains(body, `polyline points="30.00,219.00`) || !strings.Contains(body, `710.00,21.00"`) {
		t.Fatal("chart points are not chronological across the plot")
	}
	if !strings.Contains(body, "ordered newest to oldest") {
		t.Fatal("table caption does not describe newest-first order")
	}
	for _, token := range []string{"1d", "1w", "1mo", "3mo", "6mo", "1y"} {
		if !strings.Contains(body, `href="/plants/money-tree?range=`+token+`"`) {
			t.Errorf("detail page missing %s range link", token)
		}
	}
	if !strings.Contains(body, `aria-current="page">1d</a>`) || !strings.Contains(body, "Showing at most the newest 1,000 readings") {
		t.Fatal("detail page does not identify the default range and cap")
	}
}

func TestReverseReadingsCopiesWithoutMutating(t *testing.T) {
	observed := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	readings := []history.Reading{
		{ID: 1, ObservedAt: observed.Add(-time.Minute)},
		{ID: 2, ObservedAt: observed},
		{ID: 3, ObservedAt: observed},
	}
	reversed := reverseReadings(readings)
	if len(reversed) != 3 || reversed[0].ID != 3 || reversed[1].ID != 2 || reversed[2].ID != 1 {
		t.Fatalf("reversed readings = %#v", reversed)
	}
	if readings[0].ID != 1 || readings[1].ID != 2 || readings[2].ID != 3 {
		t.Fatalf("source readings mutated = %#v", readings)
	}
	if got := reverseReadings(nil); len(got) != 0 {
		t.Fatalf("nil reverse length = %d", len(got))
	}
	single := []history.Reading{{ID: 4}}
	if got := reverseReadings(single); len(got) != 1 || got[0].ID != 4 {
		t.Fatalf("single reverse = %#v", got)
	}
}

func TestDetailEmptySelectedRange(t *testing.T) {
	server, _, _ := testServer(t, 1)
	server.now = func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree?range=1d", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "No readings stored in the selected range.") || !strings.Contains(body, `aria-current="page">1d</a>`) {
		t.Fatalf("empty range response: %d %s", response.Code, body)
	}
}

func TestDetailUsesSemanticTileLayout(t *testing.T) {
	server, _, _ := testServer(t, 1)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	body := response.Body.String()

	for _, token := range []string{
		`<main class="detail-page" data-plant-id="money-tree">`,
		`class="detail-grid"`,
		`data-tile="current-moisture"`,
		`data-tile="display-name"`,
		`data-tile="photo-management"`,
		`data-tile="watering"`,
		`data-tile="moisture-history"`,
		`data-field="moisture"`,
		`data-field="raw"`,
		`data-field="observed"`,
		`action="/plants/money-tree/name"`,
		`action="/plants/money-tree/photo"`,
		`action="/plants/money-tree/water"`,
		`action="/plants/money-tree/schedule"`,
		`aria-label="Moisture history range"`,
		`role="img"`,
		`<caption>`,
	} {
		if !strings.Contains(body, token) {
			t.Errorf("detail tile layout missing %q", token)
		}
	}

	orderedTiles := []string{
		`data-tile="current-moisture"`,
		`data-tile="display-name"`,
		`data-tile="photo-management"`,
		`data-tile="watering"`,
		`data-tile="moisture-history"`,
	}
	previous := -1
	for _, tile := range orderedTiles {
		position := strings.Index(body, tile)
		if position <= previous {
			t.Fatalf("detail tile %q is out of source order", tile)
		}
		previous = position
	}
}

func TestRenameRoutes(t *testing.T) {
	server, state, _ := testServer(t, 1)
	form := url.Values{"name": {"  Money Tree  "}}
	request := httptest.NewRequest(http.MethodPost, "/plants/money-tree/name", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/plants/money-tree" || state.View().PlantName != "Money Tree" {
		t.Fatalf("rename response=%d location=%q view=%#v", response.Code, response.Header().Get("Location"), state.View())
	}

	for _, tt := range []struct {
		path, name string
		want       int
	}{{"/plants/money-tree/name", " ", 400}, {"/plants/unknown/name", "Unknown", 404}} {
		request = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(url.Values{"name": {tt.name}}.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != tt.want {
			t.Errorf("%s status=%d want=%d", tt.path, response.Code, tt.want)
		}
	}
}

func TestRoutesMethodsHeadAndErrors(t *testing.T) {
	server, state, store := testServer(t, 1)
	for _, tt := range []struct {
		method, path string
		want         int
	}{{http.MethodPost, "/", 405}, {http.MethodDelete, "/plants/money-tree", 405}, {http.MethodGet, "/unknown", 404}, {http.MethodGet, "/plants/unknown", 404}, {http.MethodGet, "/plants/bad%20id", 404}} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(tt.method, tt.path, nil))
		if response.Code != tt.want {
			t.Errorf("%s %s=%d want=%d", tt.method, tt.path, response.Code, tt.want)
		}
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/plants/money-tree", nil))
	if response.Code != 200 || response.Body.Len() != 0 {
		t.Fatalf("HEAD=%d body=%d", response.Code, response.Body.Len())
	}
	failing, err := New(":0", state, store, &testCommands{state: state, store: store, err: errors.New("disk full")})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/plants/money-tree/name", strings.NewReader("name=New"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	failing.Handler().ServeHTTP(response, request)
	if response.Code != 500 {
		t.Fatalf("storage failure=%d", response.Code)
	}
}

func TestConcurrentOverviewAndUpdates(t *testing.T) {
	server, state, store := testServer(t, 1)
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func(offset int) {
			defer group.Done()
			for j := 0; j < 100; j++ {
				reading := core.Reading{PlantID: "money-tree", Raw: offset*100 + j, Percentage: j % 101, ObservedAt: time.Now().UTC()}
				_ = store.AppendReading(context.Background(), history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt})
				state.ApplyReading(reading)
			}
		}(i)
	}
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 100; j++ {
				response := httptest.NewRecorder()
				server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
				if response.Code != 200 {
					t.Errorf("status=%d", response.Code)
					return
				}
			}
		}()
	}
	group.Wait()
}

func TestServerLifecycle(t *testing.T) {
	server, _, _ := testServer(t, 0)
	listener := &blockingListener{closed: make(chan struct{})}
	server.listen = func(_, _ string) (net.Listener, error) { return listener, nil }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketInitialSnapshotAndNotify(t *testing.T) {
	server, state, store := testServer(t, 1)

	// Start the server with a real listener so WebSocket upgrades work.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	server.listen = func(network, addr string) (net.Listener, error) {
		ln, err := net.Listen(network, addr)
		if err != nil {
			return nil, err
		}
		ready <- ln.Addr().String()
		return ln, nil
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Run(ctx) }()
	addr := <-ready

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("ws://"+addr+"/ws", nil)
	if err != nil {
		t.Fatalf("dial /ws: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Should receive the initial snapshot immediately.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read initial snapshot: %v", err)
	}
	var snap wsSnapshot
	if err := json.Unmarshal(msg, &snap); err != nil {
		t.Fatalf("unmarshal initial snapshot: %v", err)
	}
	if len(snap.Plants) != 1 || snap.Plants[0].ID != "money-tree" {
		t.Fatalf("unexpected initial snapshot: %+v", snap)
	}

	// Apply a new reading and call Notify; the client should receive an update.
	reading := core.Reading{PlantID: "money-tree", Raw: 99999, Percentage: 88, ObservedAt: time.Date(2026, 7, 1, 16, 30, 0, 0, time.UTC)}
	_ = store.AppendReading(context.Background(), history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt})
	state.ApplyPlant(core.Plant{ID: "money-tree", Name: "Money Tree", FirstSeen: reading.ObservedAt.Add(-time.Hour), LastSeen: reading.ObservedAt, Latest: &reading, Cadence: core.CadenceWeekly, NextWateringDate: "2026-07-08", HasPhoto: true})
	server.Notify(state.Snapshot())

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read notify snapshot: %v", err)
	}
	if err := json.Unmarshal(msg, &snap); err != nil {
		t.Fatalf("unmarshal notify snapshot: %v", err)
	}
	if len(snap.Plants) == 0 || snap.Plants[0].Percentage == nil || *snap.Plants[0].Percentage != 88 {
		t.Fatalf("unexpected notify snapshot: %+v", snap)
	}
	if snap.Plants[0].ObservedAt != "2026-07-01T16:30:00Z" || snap.Plants[0].ObservedAtDisplay != "2026-07-01 12:30:00 EDT" {
		t.Fatalf("unexpected notify timestamps: %+v", snap.Plants[0])
	}
	if snap.Plants[0].Cadence != "weekly" || snap.Plants[0].NextWateringDate != "2026-07-08" || !snap.Plants[0].HasPhoto || bytes.Contains(msg, []byte(`"photoBytes"`)) {
		t.Fatalf("unexpected lightweight care snapshot: %+v JSON=%s", snap.Plants[0], msg)
	}
}

func TestTemplatesContainResponsivePhotoAndFormStyles(t *testing.T) {
	server, _, _ := testServer(t, 0)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/new", nil))
	body := response.Body.String()
	for _, token := range []string{"@media (max-width: 38rem)", ".plant-photo", "object-fit: cover", "box-sizing: border-box", `accept="image/jpeg,image/png,image/heic,.heic"`} {
		if !strings.Contains(body, token) {
			t.Errorf("responsive registration template missing %q", token)
		}
	}
}

func TestDetailStylesDefineResponsiveGrid(t *testing.T) {
	server, _, _ := testServer(t, 1)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plants/money-tree", nil))
	body := response.Body.String()

	for _, token := range []string{
		`.detail-page {`,
		`.detail-grid { display: grid; grid-template-columns: minmax(0, 1fr);`,
		`.detail-tile { min-width: 0;`,
		`.history-tile { grid-column: 1 / -1;`,
		`.history-scroll { max-width: 100%; overflow-x: auto;`,
		`@media (min-width: 42rem)`,
		`grid-template-columns: repeat(2, minmax(0, 1fr));`,
		`@media (min-width: 64rem)`,
		`grid-template-columns: repeat(3, minmax(0, 1fr));`,
		`<div class="history-scroll" tabindex="0" aria-label="Scrollable moisture reading history">`,
	} {
		if !strings.Contains(body, token) {
			t.Errorf("responsive detail page missing %q", token)
		}
	}

	for _, path := range []string{"/", "/plants/new"} {
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if strings.Contains(response.Body.String(), `class="detail-grid"`) {
			t.Errorf("%s unexpectedly contains the detail grid", path)
		}
	}
}

func TestWebSocketClosedClientDoesNotPanic(t *testing.T) {
	server, state, _ := testServer(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	server.listen = func(network, addr string) (net.Listener, error) {
		ln, err := net.Listen(network, addr)
		if err != nil {
			return nil, err
		}
		ready <- ln.Addr().String()
		return ln, nil
	}
	go server.Run(ctx)
	addr := <-ready

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("ws://"+addr+"/ws", nil)
	if err != nil {
		t.Fatalf("dial /ws: %v", err)
	}

	// Consume the initial snapshot.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read initial snapshot: %v", err)
	}

	// Close the client connection, then call Notify multiple times; must not panic.
	conn.Close()
	time.Sleep(50 * time.Millisecond) // allow server to detect close via readPump
	for i := 0; i < 5; i++ {
		server.Notify(state.Snapshot())
	}
}
