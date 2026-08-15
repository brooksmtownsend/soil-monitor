package webdisplay

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"soilmonitor/internal/core"
)

func testState(t *testing.T) *core.State {
	t.Helper()
	state, err := core.NewState([]core.PlantDefinition{
		{ID: "money-tree", Name: "Money Tree"},
		{ID: "pothos", Name: "Pothos"},
		{ID: "snake-plant", Name: "Snake <Plant>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestHandlerRendersCurrentPlantStatus(t *testing.T) {
	state := testState(t)
	if _, err := state.ApplyMessage("plants/money-tree/moisture", []byte("47")); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ApplyMessage("plants/pothos/moisture", []byte("0")); err != nil {
		t.Fatal(err)
	}
	state.SetConnected(true)
	server, err := New(":8080", state)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("unexpected response: status=%d headers=%v", response.Code, response.Header())
	}
	for _, expected := range []string{"Money Tree", "47%", "Pothos", "0%", "Snake &lt;Plant&gt;", "--", "MQTT ONLINE", `http-equiv="refresh"`, `content="5"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("response missing %q", expected)
		}
	}
	if strings.Contains(body, "Snake <Plant>") || strings.Contains(strings.ToLower(body), "<script") || strings.Contains(strings.ToLower(body), "<link") {
		t.Fatalf("response contains unsafe or external markup: %s", body)
	}
}

func TestHandlerReadsFreshSnapshot(t *testing.T) {
	state := testState(t)
	server, err := New(":8080", state)
	if err != nil {
		t.Fatal(err)
	}

	first := httptest.NewRecorder()
	server.Handler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(first.Body.String(), "MQTT OFFLINE") || !strings.Contains(first.Body.String(), "--") {
		t.Fatalf("unexpected initial body: %s", first.Body.String())
	}
	if _, err := state.ApplyMessage("plants/snake-plant/moisture", []byte("63")); err != nil {
		t.Fatal(err)
	}
	state.SetConnected(true)

	second := httptest.NewRecorder()
	server.Handler().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(second.Body.String(), "63%") || !strings.Contains(second.Body.String(), "MQTT ONLINE") {
		t.Fatalf("updated state not rendered: %s", second.Body.String())
	}
}

func TestHandlerRejectsWritesAndUnknownPaths(t *testing.T) {
	state := testState(t)
	server, err := New(":8080", state)
	if err != nil {
		t.Fatal(err)
	}
	before := state.Snapshot()

	writeResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(writeResponse, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("moisture=100")))
	if writeResponse.Code != http.StatusMethodNotAllowed || writeResponse.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST response = %d, %v", writeResponse.Code, writeResponse.Header())
	}
	unknownResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(unknownResponse, httptest.NewRequest(http.MethodGet, "/api", nil))
	if unknownResponse.Code != http.StatusNotFound {
		t.Fatalf("unknown path response = %d", unknownResponse.Code)
	}
	after := state.Snapshot()
	if before.SelectedPlantID != after.SelectedPlantID || before.Connected != after.Connected || len(before.Plants) != len(after.Plants) {
		t.Fatalf("read-only requests changed state: before=%#v after=%#v", before, after)
	}
}

func TestHandlerSupportsHead(t *testing.T) {
	server, err := New(":8080", testState(t))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/", nil))
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d with %d body bytes", response.Code, response.Body.Len())
	}
}

func TestRunStopsWithContext(t *testing.T) {
	server, err := New("127.0.0.1:0", testState(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
}

func TestRunReportsListenerFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server, err := New(listener.Addr().String(), testState(t))
	if err != nil {
		t.Fatal(err)
	}
	err = server.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunServesOverHTTP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(listener.Addr().String(), testState(t))
	if err != nil {
		t.Fatal(err)
	}
	server.listen = func(_, _ string) (net.Listener, error) { return listener, nil }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()

	var response *http.Response
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		response, err = http.Get("http://" + listener.Addr().String() + "/")
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("GET root: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Soil Monitor") {
		t.Fatalf("smoke response: status=%d readErr=%v body=%s", response.StatusCode, readErr, body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() shutdown error = %v", err)
	}
}
