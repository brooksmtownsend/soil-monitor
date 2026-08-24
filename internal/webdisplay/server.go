package webdisplay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"soilmonitor/internal/core"
	"soilmonitor/internal/history"
)

const (
	refreshSeconds  = 5
	shutdownTimeout = 5 * time.Second
	historyLimit    = 1000
)

type StateSource interface {
	Snapshot() core.Snapshot
	Plant(string) (core.PlantSnapshot, bool)
}

type HistorySource interface {
	Readings(context.Context, string, int) ([]history.Reading, error)
}

type PlantRenamer interface {
	RenamePlant(context.Context, string, string) error
}

type Server struct {
	address         string
	state           StateSource
	history         HistorySource
	renamer         PlantRenamer
	handler         http.Handler
	listen          func(network, address string) (net.Listener, error)
	shutdownTimeout time.Duration
}

func New(address string, state StateSource, historySource HistorySource, renamer PlantRenamer) (*Server, error) {
	if state == nil || historySource == nil || renamer == nil {
		return nil, fmt.Errorf("state, history, and plant renamer are required")
	}
	overview, err := template.New("overview").Parse(overviewHTML)
	if err != nil {
		return nil, fmt.Errorf("parse overview template: %w", err)
	}
	detail, err := template.New("detail").Parse(detailHTML)
	if err != nil {
		return nil, fmt.Errorf("parse detail template: %w", err)
	}
	server := &Server{address: address, state: state, history: historySource, renamer: renamer, listen: net.Listen, shutdownTimeout: shutdownTimeout}
	server.handler = server.newHandler(overview, detail)
	return server, nil
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) Run(ctx context.Context) error {
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

func (s *Server) newHandler(overview, detail *template.Template) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/":
			s.renderOverview(response, request, overview)
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
	http.NotFound(response, request)
}

func (s *Server) renderPlant(response http.ResponseWriter, request *http.Request, page *template.Template, id string) {
	plant, ok := s.state.Plant(id)
	if !ok {
		http.NotFound(response, request)
		return
	}
	readings, err := s.history.Readings(request.Context(), id, historyLimit)
	if errors.Is(err, history.ErrPlantNotFound) {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.Error(response, "read plant history", http.StatusInternalServerError)
		return
	}
	data := detailData{Plant: plant, Readings: readings, ChartPoints: chartPoints(readings)}
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
	if err := s.renamer.RenamePlant(request.Context(), id, name); errors.Is(err, history.ErrPlantNotFound) {
		http.NotFound(response, request)
		return
	} else if err != nil {
		http.Error(response, "store plant name", http.StatusInternalServerError)
		return
	}
	http.Redirect(response, request, "/plants/"+id, http.StatusSeeOther)
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
	Plant       core.PlantSnapshot
	Readings    []history.Reading
	ChartPoints template.HTMLAttr
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

const sharedCSS = `
    :root { color-scheme: light dark; font-family: system-ui, sans-serif; }
    body { margin: 0; background: #18221a; color: #f4f7f2; }
    main { width: min(60rem, calc(100% - 2rem)); margin: 3rem auto; }
    header { display: flex; justify-content: space-between; align-items: baseline; gap: 1rem; }
    a { color: #bde7bd; } .status { padding: .4rem .7rem; border-radius: 999px; font-weight: 700; }
    .online { background: #2c7a45; } .offline { background: #8d3030; }
    .plants { display: grid; grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr)); gap: 1rem; margin-top: 2rem; }
    .plant, .panel { padding: 1.25rem; border: 1px solid #526456; border-radius: .8rem; background: #223126; }
    .plant.selected { border-color: #9ed59f; } .plant h2 { margin: 0; font-size: 1.2rem; }
    .moisture { margin: .5rem 0; font-size: 3rem; font-variant-numeric: tabular-nums; }
    .muted { color: #b8c8ba; } table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: .5rem; border-bottom: 1px solid #526456; font-variant-numeric: tabular-nums; }
    svg { width: 100%; height: auto; background: #152018; border-radius: .5rem; }
    input, button { font: inherit; padding: .55rem; }`

const overviewHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="refresh" content="5"><title>Soil Monitor</title><style>` + sharedCSS + `</style></head><body><main>
<header><h1>Soil Monitor</h1>{{if .Connected}}<p class="status online">MQTT ONLINE</p>{{else}}<p class="status offline">MQTT OFFLINE</p>{{end}}</header>
{{if .Plants}}<section class="plants" aria-label="Plant moisture readings">{{range .Plants}}<article class="plant{{if eq $.SelectedPlantID .ID}} selected{{end}}"><h2><a href="/plants/{{.ID}}">{{.Name}}</a></h2>{{if .Latest}}<p class="moisture">{{.Latest.Percentage}}%</p><p>Raw: {{.Latest.Raw}}</p><p><time datetime="{{.Latest.ObservedAt.Format "2006-01-02T15:04:05Z07:00"}}">{{.Latest.ObservedAt.Format "2006-01-02 15:04:05 MST"}}</time></p>{{else}}<p class="moisture">--</p><p>Reading unknown</p>{{end}}<p class="muted">{{.ID}}</p></article>{{end}}</section>{{else}}<section class="panel"><h2>Waiting for readings</h2><p>Plants appear automatically after valid MQTT readings arrive.</p></section>{{end}}
</main></body></html>`

const detailHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="refresh" content="5"><title>{{.Plant.Name}} — Soil Monitor</title><style>` + sharedCSS + `</style></head><body><main>
<p><a href="/">← All plants</a></p><header><div><h1>{{.Plant.Name}}</h1><p class="muted">{{.Plant.ID}}</p></div>{{if .Plant.Latest}}<p class="moisture">{{.Plant.Latest.Percentage}}%</p>{{end}}</header>
<section class="panel"><h2>Display name</h2><form method="post" action="/plants/{{.Plant.ID}}/name"><label>Name <input name="name" value="{{.Plant.Name}}" maxlength="80" required></label> <button type="submit">Save</button></form></section>
<section class="panel"><h2>Moisture history</h2>{{if .Readings}}<svg viewBox="0 0 720 250" role="img" aria-labelledby="chart-title chart-desc"><title id="chart-title">Moisture percentage history</title><desc id="chart-desc">Chronological readings from zero to one hundred percent.</desc><line x1="30" y1="10" x2="30" y2="230" stroke="currentColor"/><line x1="30" y1="230" x2="710" y2="230" stroke="currentColor"/><polyline points="{{.ChartPoints}}" fill="none" stroke="#9ed59f" stroke-width="3"/></svg><table><caption>Newest {{len .Readings}} readings, ordered oldest to newest</caption><thead><tr><th>Observed</th><th>Raw</th><th>Moisture</th></tr></thead><tbody>{{range .Readings}}<tr><td><time datetime="{{.ObservedAt.Format "2006-01-02T15:04:05Z07:00"}}">{{.ObservedAt.Format "2006-01-02 15:04:05 MST"}}</time></td><td>{{.Raw}}</td><td>{{.Percentage}}%</td></tr>{{end}}</tbody></table>{{else}}<p>No readings stored.</p>{{end}}</section>
</main></body></html>`
