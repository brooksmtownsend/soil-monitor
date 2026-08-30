package webdisplay

import (
	"context"
	"encoding/json"
	"errors"
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

type testRenamer struct {
	state *core.State
	store *history.Store
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

func (r *testRenamer) RenamePlant(ctx context.Context, id, name string) error {
	if r.err != nil {
		return r.err
	}
	if err := r.store.RenamePlant(ctx, id, name); err != nil {
		return err
	}
	r.state.RenamePlant(id, name)
	return nil
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
	server, err := New(":0", state, store, &testRenamer{state: state, store: store})
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

func TestDetailRangeRequests(t *testing.T) {
	now := time.Date(2028, 3, 31, 18, 15, 16, 0, time.UTC)
	state, _ := core.NewState(nil)
	state.ApplyReading(core.Reading{PlantID: "money-tree", Raw: 1, Percentage: 2, ObservedAt: now})
	historySource := &recordingHistory{}
	server, err := New(":0", state, historySource, &testRenamer{})
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
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Waiting for readings") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("empty response: %d %s", response.Code, response.Body.String())
	}
	reading := core.Reading{PlantID: "money-tree", Raw: 32768, Percentage: 47, ObservedAt: time.Now().UTC()}
	if err := store.AppendReading(context.Background(), history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt}); err != nil {
		t.Fatal(err)
	}
	state.ApplyReading(reading)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, text := range []string{"money-tree", "47%", "Raw: 32768", `/plants/money-tree`} {
		if !strings.Contains(body, text) {
			t.Fatalf("overview missing %q: %s", text, body)
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
	if strings.Index(body, ">30005<") > strings.Index(body, ">31004<") {
		t.Fatal("history is not chronological")
	}
	if !strings.Contains(body, `polyline points="30.00,`) || !strings.Contains(body, `710.00,`) {
		t.Fatal("chart points do not span the plot")
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
	failing, err := New(":0", state, store, &testRenamer{state: state, store: store, err: errors.New("disk full")})
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
	reading := core.Reading{PlantID: "money-tree", Raw: 99999, Percentage: 88, ObservedAt: time.Now().UTC()}
	_ = store.AppendReading(context.Background(), history.Reading{PlantID: reading.PlantID, Raw: reading.Raw, Percentage: reading.Percentage, ObservedAt: reading.ObservedAt})
	state.ApplyReading(reading)
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
