package webdisplay

import (
	"context"
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
	return server, state, store
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
