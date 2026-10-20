package webdisplay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"soilmonitor/internal/core"
	"soilmonitor/internal/history"
)

const (
	shutdownTimeout = 5 * time.Second
	historyLimit    = 1000
	multipartMemory = 1 << 20
	multipartLimit  = core.MaxPhotoBytes + multipartMemory
)

type StateSource interface {
	Snapshot() core.Snapshot
	Plant(string) (core.PlantSnapshot, bool)
}

type HistorySource interface {
	Readings(context.Context, string, time.Time, time.Time, int) ([]history.Reading, error)
	Photo(context.Context, string) (history.Photo, error)
}

type PlantCommands interface {
	RenamePlant(context.Context, string, string) error
	RegisterPlant(context.Context, string, core.Cadence, *history.Photo) (string, error)
	SetSchedule(context.Context, string, core.Cadence) error
	ConfirmWatering(context.Context, string) error
	ReplacePhoto(context.Context, string, history.Photo) error
}

type Server struct {
	address         string
	state           StateSource
	history         HistorySource
	commands        PlantCommands
	handler         http.Handler
	listen          func(network, address string) (net.Listener, error)
	shutdownTimeout time.Duration
	hub             *hub
	now             func() time.Time
	uploadSlots     chan struct{}
}

func New(address string, state StateSource, historySource HistorySource, commands PlantCommands) (*Server, error) {
	if state == nil || historySource == nil || commands == nil {
		return nil, fmt.Errorf("state, history, and plant commands are required")
	}
	templateFunctions := template.FuncMap{
		"easternTime": formatEasternTime,
		"machineTime": formatMachineTime,
	}
	overview, err := template.New("overview").Funcs(templateFunctions).Parse(overviewHTML)
	if err != nil {
		return nil, fmt.Errorf("parse overview template: %w", err)
	}
	detail, err := template.New("detail").Funcs(templateFunctions).Parse(detailHTML)
	if err != nil {
		return nil, fmt.Errorf("parse detail template: %w", err)
	}
	registration, err := template.New("registration").Funcs(templateFunctions).Parse(registrationHTML)
	if err != nil {
		return nil, fmt.Errorf("parse registration template: %w", err)
	}
	server := &Server{address: address, state: state, history: historySource, commands: commands, listen: net.Listen, shutdownTimeout: shutdownTimeout, hub: newHub(), now: time.Now, uploadSlots: make(chan struct{}, 1)}
	server.handler = server.newHandler(overview, detail, registration)
	return server, nil
}

func (s *Server) Handler() http.Handler { return s.handler }

// Notify pushes a state snapshot to all connected WebSocket clients.
// It is safe to call from any goroutine. If the hub is busy, the update is dropped.
func (s *Server) Notify(snap core.Snapshot) {
	msg, err := marshalSnapshot(snap)
	if err != nil {
		return
	}
	select {
	case s.hub.broadcast <- msg:
	default:
	}
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	initial, _ := marshalSnapshot(s.state.Snapshot())
	c := &wsClient{hub: s.hub, conn: conn, send: make(chan []byte, 8)}
	s.hub.register <- hubRegistration{client: c, initial: initial}
	go c.writePump()
	go c.readPump()
}

func (s *Server) Run(ctx context.Context) error {
	go s.hub.run(ctx.Done())
	listener, err := s.listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.address, err)
	}
	httpServer := &http.Server{Handler: s.handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.Serve(listener) }()
	select {
	case serveErr := <-serveErrors:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP on %s: %w", s.address, serveErr)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		serveErr := <-serveErrors
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP on %s: %w", s.address, serveErr)
		}
		return nil
	}
}

func (s *Server) newHandler(overview, detail, registration *template.Template) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/":
			s.renderOverview(response, request, overview)
		case request.URL.Path == "/ws":
			s.serveWS(response, request)
		case request.URL.Path == "/plants/new":
			if !readMethod(response, request) {
				return
			}
			writeHTML(response, request, registration, nil)
		case request.URL.Path == "/plants":
			s.registerPlant(response, request)
		case strings.HasPrefix(request.URL.Path, "/plants/"):
			s.handlePlant(response, request, detail)
		default:
			http.NotFound(response, request)
		}
	})
}

func (s *Server) renderOverview(response http.ResponseWriter, request *http.Request, page *template.Template) {
	if !readMethod(response, request) {
		return
	}
	writeHTML(response, request, page, s.state.Snapshot())
}

func (s *Server) handlePlant(response http.ResponseWriter, request *http.Request, page *template.Template) {
	path := strings.TrimPrefix(request.URL.Path, "/plants/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && core.ValidPlantID(parts[0]) {
		if !readMethod(response, request) {
			return
		}
		s.renderPlant(response, request, page, parts[0])
		return
	}
	if len(parts) == 2 && core.ValidPlantID(parts[0]) && parts[1] == "name" {
		s.renamePlant(response, request, parts[0])
		return
	}
	if len(parts) == 2 && core.ValidPlantID(parts[0]) {
		switch parts[1] {
		case "schedule":
			s.setSchedule(response, request, parts[0])
		case "water":
			s.confirmWatering(response, request, parts[0])
		case "photo":
			s.handlePhoto(response, request, parts[0])
		default:
			http.NotFound(response, request)
		}
		return
	}
	http.NotFound(response, request)
}

func (s *Server) renderPlant(response http.ResponseWriter, request *http.Request, page *template.Template, id string) {
	preset, err := parseHistoryRange(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	plant, ok := s.state.Plant(id)
	if !ok {
		http.NotFound(response, request)
		return
	}
	end := s.now().UTC()
	start := preset.start(end)
	readings, err := s.history.Readings(request.Context(), id, start, end, historyLimit)
	if errors.Is(err, history.ErrPlantNotFound) {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.Error(response, "read plant history", http.StatusInternalServerError)
		return
	}
	data := detailData{
		Plant:         plant,
		Readings:      readings,
		TableReadings: reverseReadings(readings),
		ChartPoints:   chartPoints(readings),
		Ranges:        historyRangeOptions(id, preset.Key),
		ActiveRange:   preset.Key,
		RangeStart:    start,
		RangeEnd:      end,
	}
	writeHTML(response, request, page, data)
}

func (s *Server) renamePlant(response http.ResponseWriter, request *http.Request, id string) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.state.Plant(id); !ok {
		http.NotFound(response, request)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(response, "invalid form", http.StatusBadRequest)
		return
	}
	name, err := core.NormalizePlantName(request.FormValue("name"))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.commands.RenamePlant(request.Context(), id, name); errors.Is(err, history.ErrPlantNotFound) {
		http.NotFound(response, request)
		return
	} else if err != nil {
		http.Error(response, "store plant name", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/plants/"+id, http.StatusSeeOther)
}

func (s *Server) registerPlant(response http.ResponseWriter, request *http.Request) {
	if !postMethod(response, request) {
		return
	}
	if !s.beginUpload(response, request) {
		return
	}
	defer func() { <-s.uploadSlots }()
	photo, ok := s.parsePhotoForm(response, request, true)
	if !ok {
		return
	}
	name := request.FormValue("name")
	if _, _, err := core.DerivePlantID(name); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	cadence := core.Cadence(request.FormValue("cadence"))
	if !core.ValidCadence(cadence) {
		http.Error(response, "unsupported watering cadence", http.StatusBadRequest)
		return
	}
	id, err := s.commands.RegisterPlant(request.Context(), name, cadence, photo)
	if errors.Is(err, history.ErrPlantConflict) {
		http.Error(response, "plant identifier already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(response, "store plant registration", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/plants/"+id, http.StatusSeeOther)
}

func (s *Server) setSchedule(response http.ResponseWriter, request *http.Request, id string) {
	if !postMethod(response, request) {
		return
	}
	if _, ok := s.state.Plant(id); !ok {
		http.NotFound(response, request)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(response, "invalid form", http.StatusBadRequest)
		return
	}
	cadence := core.Cadence(request.FormValue("cadence"))
	if !core.ValidCadence(cadence) {
		http.Error(response, "unsupported watering cadence", http.StatusBadRequest)
		return
	}
	if err := s.commands.SetSchedule(request.Context(), id, cadence); !writeMutationResult(response, request, id, "store watering schedule", err) {
		return
	}
}

func (s *Server) confirmWatering(response http.ResponseWriter, request *http.Request, id string) {
	if !postMethod(response, request) {
		return
	}
	if _, ok := s.state.Plant(id); !ok {
		http.NotFound(response, request)
		return
	}
	_ = writeMutationResult(response, request, id, "store watering confirmation", s.commands.ConfirmWatering(request.Context(), id))
}

func (s *Server) handlePhoto(response http.ResponseWriter, request *http.Request, id string) {
	switch request.Method {
	case http.MethodGet, http.MethodHead:
		s.servePhoto(response, request, id)
	case http.MethodPost:
		if _, ok := s.state.Plant(id); !ok {
			http.NotFound(response, request)
			return
		}
		if !s.beginUpload(response, request) {
			return
		}
		defer func() { <-s.uploadSlots }()
		photo, ok := s.parsePhotoForm(response, request, false)
		if !ok {
			return
		}
		if photo == nil {
			http.Error(response, "photo is required", http.StatusUnsupportedMediaType)
			return
		}
		_ = writeMutationResult(response, request, id, "store plant photo", s.commands.ReplacePhoto(request.Context(), id, *photo))
	default:
		response.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) servePhoto(response http.ResponseWriter, request *http.Request, id string) {
	photo, err := s.history.Photo(request.Context(), id)
	if errors.Is(err, history.ErrPlantNotFound) || errors.Is(err, history.ErrPhotoNotFound) {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.Error(response, "read plant photo", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", photo.MediaType)
	response.Header().Set("Content-Length", fmt.Sprintf("%d", len(photo.Bytes)))
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = response.Write(photo.Bytes)
	}
}

func (s *Server) parsePhotoForm(response http.ResponseWriter, request *http.Request, optional bool) (*history.Photo, bool) {
	request.Body = http.MaxBytesReader(response, request.Body, multipartLimit)
	if err := request.ParseMultipartForm(multipartMemory); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(response, "photo is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(response, "invalid multipart form", http.StatusBadRequest)
		}
		return nil, false
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	file, _, err := request.FormFile("photo")
	if errors.Is(err, http.ErrMissingFile) && optional {
		return nil, true
	}
	if err != nil {
		http.Error(response, "photo is required", http.StatusUnsupportedMediaType)
		return nil, false
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, core.MaxPhotoBytes+1))
	if err != nil {
		http.Error(response, "read photo", http.StatusBadRequest)
		return nil, false
	}
	if len(content) > core.MaxPhotoBytes {
		http.Error(response, "photo is too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	mediaType, err := core.DetectPhotoMediaType(content)
	if err != nil {
		http.Error(response, err.Error(), http.StatusUnsupportedMediaType)
		return nil, false
	}
	return &history.Photo{Bytes: content, MediaType: mediaType}, true
}

func (s *Server) beginUpload(response http.ResponseWriter, request *http.Request) bool {
	select {
	case s.uploadSlots <- struct{}{}:
		return true
	case <-request.Context().Done():
		http.Error(response, "request canceled", http.StatusRequestTimeout)
		return false
	}
}

func writeMutationResult(response http.ResponseWriter, request *http.Request, id, message string, err error) bool {
	if errors.Is(err, history.ErrPlantNotFound) {
		http.NotFound(response, request)
		return false
	}
	if err != nil {
		http.Error(response, message, http.StatusInternalServerError)
		return false
	}
	http.Redirect(response, request, "/plants/"+id, http.StatusSeeOther)
	return true
}

func postMethod(response http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodPost {
		return true
	}
	response.Header().Set("Allow", http.MethodPost)
	http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func readMethod(response http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return true
	}
	response.Header().Set("Allow", "GET, HEAD")
	http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func writeHTML(response http.ResponseWriter, request *http.Request, page *template.Template, data any) {
	var body bytes.Buffer
	if err := page.Execute(&body, data); err != nil {
		http.Error(response, "render page", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = response.Write(body.Bytes())
	}
}

type detailData struct {
	Plant         core.PlantSnapshot
	Readings      []history.Reading
	TableReadings []history.Reading
	ChartPoints   template.HTMLAttr
	Ranges        []historyRangeOption
	ActiveRange   string
	RangeStart    time.Time
	RangeEnd      time.Time
}

type historyRangePreset struct {
	Key      string
	Duration time.Duration
	Months   int
}

type historyRangeOption struct {
	Key    string
	Href   string
	Active bool
}

var historyRangePresets = []historyRangePreset{
	{Key: "1d", Duration: 24 * time.Hour},
	{Key: "1w", Duration: 7 * 24 * time.Hour},
	{Key: "1mo", Months: 1},
	{Key: "3mo", Months: 3},
	{Key: "6mo", Months: 6},
	{Key: "1y", Months: 12},
}

func parseHistoryRange(request *http.Request) (historyRangePreset, error) {
	values, present := request.URL.Query()["range"]
	key := "1d"
	if present {
		if len(values) != 1 || values[0] == "" {
			return historyRangePreset{}, fmt.Errorf("invalid history range")
		}
		key = values[0]
	}
	for _, preset := range historyRangePresets {
		if preset.Key == key {
			return preset, nil
		}
	}
	return historyRangePreset{}, fmt.Errorf("unsupported history range %q", key)
}

func (p historyRangePreset) start(end time.Time) time.Time {
	if p.Duration != 0 {
		return end.Add(-p.Duration)
	}
	return subtractCalendarMonths(end, p.Months)
}

func subtractCalendarMonths(value time.Time, months int) time.Time {
	value = value.UTC()
	targetMonth := time.Date(value.Year(), value.Month(), 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC).AddDate(0, -months, 0)
	lastDay := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC).Day()
	day := value.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetMonth.Year(), targetMonth.Month(), day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func historyRangeOptions(plantID, active string) []historyRangeOption {
	options := make([]historyRangeOption, len(historyRangePresets))
	for i, preset := range historyRangePresets {
		options[i] = historyRangeOption{Key: preset.Key, Href: "/plants/" + plantID + "?range=" + preset.Key, Active: preset.Key == active}
	}
	return options
}

func reverseReadings(readings []history.Reading) []history.Reading {
	reversed := make([]history.Reading, len(readings))
	for i := range readings {
		reversed[len(readings)-1-i] = readings[i]
	}
	return reversed
}

func chartPoints(readings []history.Reading) template.HTMLAttr {
	if len(readings) == 0 {
		return ""
	}
	const left, right, top, bottom = 30.0, 710.0, 10.0, 230.0
	points := make([]string, len(readings))
	for i, reading := range readings {
		x := (left + right) / 2
		if len(readings) > 1 {
			x = left + float64(i)*(right-left)/float64(len(readings)-1)
		}
		y := bottom - float64(reading.Percentage)*(bottom-top)/100
		points[i] = fmt.Sprintf("%.2f,%.2f", x, y)
	}
	return template.HTMLAttr(strings.Join(points, " "))
}

const wsScript = `<script>(function(){var ws,delay=1000;function connect(){var proto=location.protocol==='https:'?'wss:':'ws:';ws=new WebSocket(proto+'//'+location.host+'/ws');ws.onopen=function(){delay=1000;};ws.onmessage=function(e){var d;try{d=JSON.parse(e.data);}catch(_){return;}var s=document.querySelector('[data-mqtt-status]');if(s){s.textContent=d.connected?'MQTT ONLINE':'MQTT OFFLINE';s.className='status '+(d.connected?'online':'offline');}(d.plants||[]).forEach(function(p){var c=document.querySelector('[data-plant-id="'+p.id+'"]');if(!c)return;var m=c.querySelector('[data-field="moisture"]');if(m)m.textContent=p.percentage!=null?p.percentage+'%':'--';var r=c.querySelector('[data-field="raw"]');if(r)r.textContent=p.raw!=null?'Raw: '+p.raw:'Reading unknown';var t=c.querySelector('[data-field="observed"]');if(t&&p.observedAt){t.setAttribute('datetime',p.observedAt);t.textContent=p.observedAtDisplay||p.observedAt;}var n=c.querySelector('[data-field="next-watering"]');if(n){if(p.nextWateringDate){n.textContent='Next watering: '+p.nextWateringDate;}else if(p.cadence==='as-needed'){n.textContent='Water as needed';}else{n.textContent='Watering is not scheduled';}}});};ws.onclose=function(){delay=Math.min(delay*2,30000);setTimeout(connect,delay);};}connect();})();</script>`

const sharedCSS = `
    :root { color-scheme: light dark; font-family: system-ui, sans-serif; }
    body { margin: 0; background: #18221a; color: #f4f7f2; }
    main { width: min(60rem, calc(100% - 2rem)); margin: 3rem auto; }
    header, .actions { display: flex; justify-content: space-between; align-items: center; gap: 1rem; flex-wrap: wrap; }
    a { color: #bde7bd; } .button, button { border: 1px solid #7da47f; border-radius: .4rem; background: #2c7a45; color: #fff; cursor: pointer; text-decoration: none; }
    .button { display: inline-block; padding: .55rem .8rem; } .status { padding: .4rem .7rem; border-radius: 999px; font-weight: 700; }
    .online { background: #2c7a45; } .offline { background: #8d3030; }
    .plants { display: grid; grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr)); gap: 1rem; margin-top: 2rem; }
    .plant, .panel { padding: 1.25rem; border: 1px solid #526456; border-radius: .8rem; background: #223126; }
    .plant h2 { margin: .75rem 0 0; font-size: 1.2rem; }
	.plant-photo { display: block; width: 100%; max-height: 20rem; aspect-ratio: 4 / 3; object-fit: cover; border-radius: .55rem; background: #152018; }
	.detail-photo { max-width: none; max-height: 18rem; margin: 0 0 1rem; }
    .moisture { margin: .5rem 0; font-size: 3rem; font-variant-numeric: tabular-nums; }
    .muted { color: #b8c8ba; } table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: .5rem; border-bottom: 1px solid #526456; font-variant-numeric: tabular-nums; }
    svg { width: 100%; height: auto; background: #152018; border-radius: .5rem; }
	form { display: grid; gap: .8rem; } label { display: grid; gap: .35rem; } input, select, button { font: inherit; padding: .55rem; }
	input, select { max-width: 32rem; } button { width: fit-content; }
    .ranges { display: flex; flex-wrap: wrap; gap: .5rem; margin: 1rem 0; }
    .ranges a { border: 1px solid #526456; border-radius: 999px; padding: .35rem .7rem; text-decoration: none; }
    .ranges a[aria-current="page"] { background: #2c7a45; border-color: #9ed59f; color: #fff; }
	.detail-page { width: min(72rem, calc(100% - 2rem)); }
	.detail-nav { margin: 0 0 1.5rem; }
	.detail-nav a { text-decoration: none; }
	.detail-header { margin-bottom: 1.5rem; align-items: end; }
	.detail-header h1 { margin: 0; font-size: clamp(2rem, 5vw, 3.25rem); line-height: 1.05; letter-spacing: -.03em; }
	.detail-kicker { margin: 0 0 .4rem; color: #9ed59f; font-size: .8rem; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
	.detail-header .muted { margin: .55rem 0 0; }
	.detail-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; align-items: start; }
	.detail-tile { min-width: 0; box-sizing: border-box; border-color: #5d7161; box-shadow: 0 .75rem 2rem rgb(5 15 8 / 18%); }
	.detail-tile h2 { margin: 0 0 1rem; font-size: 1.05rem; letter-spacing: .01em; }
	.detail-tile form + form { margin-top: 1rem; padding-top: 1rem; border-top: 1px solid #526456; }
	.detail-tile input, .detail-tile select { width: 100%; max-width: 100%; box-sizing: border-box; border: 1px solid #647867; border-radius: .45rem; background: #152018; color: #f4f7f2; }
	.detail-tile input[type="file"] { padding: .35rem; color: #b8c8ba; }
	.detail-tile input[type="file"]::file-selector-button { margin-right: .65rem; padding: .4rem .55rem; border: 0; border-radius: .3rem; background: #344c39; color: #f4f7f2; cursor: pointer; }
	.detail-tile button:hover, .detail-tile input[type="file"]::file-selector-button:hover { background: #348d50; }
	.detail-tile :focus-visible { outline: 2px solid #9ed59f; outline-offset: 2px; }
	.moisture-tile { background: linear-gradient(145deg, #29432f, #223126 68%); }
	.moisture-tile .moisture { margin: .15rem 0 .35rem; font-size: clamp(3.25rem, 8vw, 4.75rem); font-weight: 750; line-height: 1; color: #d8f4d8; }
	.reading-meta { margin: .45rem 0 0; }
	.photo-tile .muted { margin: 0; font-size: .9rem; }
	.history-tile { grid-column: 1 / -1; }
	.history-tile svg { margin-top: .5rem; }
	.history-scroll { max-width: 100%; overflow-x: auto; margin-top: 1rem; border-radius: .5rem; overscroll-behavior-inline: contain; }
	.history-scroll:focus-visible { outline: 2px solid #9ed59f; outline-offset: 2px; }
	.history-scroll table { min-width: 34rem; }
	.history-scroll caption { padding: 0 0 .65rem; text-align: left; color: #b8c8ba; }
	@media (min-width: 42rem) { .detail-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
	@media (min-width: 64rem) { .detail-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
	@media (max-width: 38rem) { main { margin: 1rem auto; } .plants { grid-template-columns: 1fr; } input, select, button { width: 100%; max-width: 100%; box-sizing: border-box; } }`

const overviewHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Soil Monitor</title><style>` + sharedCSS + `</style></head><body><main>
<header><h1>Soil Monitor</h1><div class="actions"><a class="button" href="/plants/new">Add plant</a>{{if .Connected}}<p class="status online" data-mqtt-status>MQTT ONLINE</p>{{else}}<p class="status offline" data-mqtt-status>MQTT OFFLINE</p>{{end}}</div></header>
{{if .Plants}}<section class="plants" aria-label="Plant moisture readings">{{range .Plants}}<article class="plant" data-plant-id="{{.ID}}">{{if .HasPhoto}}<img class="plant-photo" src="/plants/{{.ID}}/photo" alt="Photo of {{.Name}}">{{end}}<h2><a href="/plants/{{.ID}}">{{.Name}}</a></h2>{{if .Latest}}<p class="moisture" data-field="moisture">{{.Latest.Percentage}}%</p><p data-field="raw">Raw: {{.Latest.Raw}}</p><p><time data-field="observed" datetime="{{machineTime .Latest.ObservedAt}}">{{easternTime .Latest.ObservedAt}}</time></p>{{else}}<p class="moisture" data-field="moisture">--</p><p data-field="raw">Reading unknown</p>{{end}}<p data-field="next-watering">{{if .NextWateringDate}}Next watering: {{.NextWateringDate}}{{else if eq .Cadence "as-needed"}}Water as needed{{else}}Watering is not scheduled{{end}}</p><p class="muted">{{.ID}}</p></article>{{end}}</section>{{else}}<section class="panel"><h2>No plants yet</h2><p><a href="/plants/new">Register a plant</a> or wait for a valid MQTT reading to discover one automatically.</p></section>{{end}}
</main>` + wsScript + `</body></html>`

const detailHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Plant.Name}} — Soil Monitor</title><style>` + sharedCSS + `</style></head><body><main class="detail-page" data-plant-id="{{.Plant.ID}}">
<p class="detail-nav"><a href="/">← All plants</a></p><header class="detail-header"><div><p class="detail-kicker">Plant details</p><h1>{{.Plant.Name}}</h1><p class="muted">{{.Plant.ID}}</p></div></header>
<div class="detail-grid">
<section class="panel detail-tile moisture-tile" data-tile="current-moisture"><h2>Current moisture</h2>{{if .Plant.Latest}}<p class="moisture" data-field="moisture">{{.Plant.Latest.Percentage}}%</p><p class="reading-meta" data-field="raw">Raw: {{.Plant.Latest.Raw}}</p><p class="muted reading-meta"><time data-field="observed" datetime="{{machineTime .Plant.Latest.ObservedAt}}">{{easternTime .Plant.Latest.ObservedAt}}</time></p>{{else}}<p class="moisture" data-field="moisture">--</p><p class="reading-meta" data-field="raw">Reading unknown</p><p class="muted reading-meta"><time data-field="observed">No reading received yet</time></p>{{end}}</section>
<section class="panel detail-tile name-tile" data-tile="display-name"><h2>Display name</h2><form method="post" action="/plants/{{.Plant.ID}}/name"><label>Name <input name="name" value="{{.Plant.Name}}" maxlength="80" required></label><button type="submit">Save name</button></form></section>
<section class="panel detail-tile photo-tile" data-tile="photo-management"><h2>Photo</h2>{{if .Plant.HasPhoto}}<img class="plant-photo detail-photo" src="/plants/{{.Plant.ID}}/photo" alt="Photo of {{.Plant.Name}}">{{end}}<form method="post" action="/plants/{{.Plant.ID}}/photo" enctype="multipart/form-data"><label>{{if .Plant.HasPhoto}}Replace photo{{else}}Add photo{{end}} <input type="file" name="photo" accept="image/jpeg,image/png,image/heic,.heic" required></label><p class="muted">JPEG, PNG, or HEIC, up to 20 MiB. JPEG works in the widest range of browsers.</p><button type="submit">Upload photo</button></form></section>
<section class="panel detail-tile watering-tile" data-tile="watering"><h2>Watering</h2><p data-field="next-watering">{{if .Plant.NextWateringDate}}Next watering: <time datetime="{{.Plant.NextWateringDate}}">{{.Plant.NextWateringDate}}</time>{{else if eq .Plant.Cadence "as-needed"}}Water as needed{{else}}Watering is not scheduled{{end}}</p><form method="post" action="/plants/{{.Plant.ID}}/water"><button type="submit">Confirm watered</button></form><form method="post" action="/plants/{{.Plant.ID}}/schedule"><label>Watering schedule <select name="cadence" required><option value="weekly"{{if eq .Plant.Cadence "weekly"}} selected{{end}}>Weekly</option><option value="biweekly"{{if eq .Plant.Cadence "biweekly"}} selected{{end}}>Biweekly</option><option value="monthly"{{if eq .Plant.Cadence "monthly"}} selected{{end}}>Monthly</option><option value="as-needed"{{if eq .Plant.Cadence "as-needed"}} selected{{end}}>As needed</option></select></label><button type="submit">Save schedule</button></form></section>
<section class="panel detail-tile history-tile" data-tile="moisture-history"><h2>Moisture history</h2><nav class="ranges" aria-label="Moisture history range">{{range .Ranges}}<a href="{{.Href}}"{{if .Active}} aria-current="page"{{end}}>{{.Key}}</a>{{end}}</nav><p class="muted">Selected {{.ActiveRange}} range: <time datetime="{{machineTime .RangeStart}}">{{easternTime .RangeStart}}</time> through <time datetime="{{machineTime .RangeEnd}}">{{easternTime .RangeEnd}}</time>. Showing at most the newest 1,000 readings.</p>{{if .Readings}}<svg viewBox="0 0 720 250" role="img" aria-labelledby="chart-title chart-desc"><title id="chart-title">Moisture percentage history</title><desc id="chart-desc">Chronological readings from zero to one hundred percent in the selected range.</desc><line x1="30" y1="10" x2="30" y2="230" stroke="currentColor"/><line x1="30" y1="230" x2="710" y2="230" stroke="currentColor"/><polyline points="{{.ChartPoints}}" fill="none" stroke="#9ed59f" stroke-width="3"/></svg><div class="history-scroll" tabindex="0" aria-label="Scrollable moisture reading history"><table><caption>Newest {{len .Readings}} readings in the selected range, ordered newest to oldest</caption><thead><tr><th>Observed</th><th>Raw</th><th>Moisture</th></tr></thead><tbody>{{range .TableReadings}}<tr><td><time datetime="{{machineTime .ObservedAt}}">{{easternTime .ObservedAt}}</time></td><td>{{.Raw}}</td><td>{{.Percentage}}%</td></tr>{{end}}</tbody></table></div>{{else}}<p>No readings stored in the selected range.</p>{{end}}</section>
</div></main>` + wsScript + `</body></html>`

const registrationHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Add plant — Soil Monitor</title><style>` + sharedCSS + `</style></head><body><main>
<p><a href="/">← All plants</a></p><h1>Add plant</h1><section class="panel"><form method="post" action="/plants" enctype="multipart/form-data"><label>Name <input name="name" maxlength="80" required></label><p class="muted">The permanent MQTT and database ID is generated from this name in lowercase kebab case.</p><label>Watering schedule <select name="cadence" required><option value="weekly">Weekly</option><option value="biweekly">Biweekly</option><option value="monthly">Monthly</option><option value="as-needed">As needed</option></select></label><label>Photo (optional) <input type="file" name="photo" accept="image/jpeg,image/png,image/heic,.heic"></label><p class="muted">JPEG, PNG, or HEIC, up to 20 MiB. JPEG works in the widest range of browsers.</p><button type="submit">Add plant</button></form></section>
</main></body></html>`
