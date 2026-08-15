package webdisplay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"soilmonitor/internal/core"
)

const (
	refreshSeconds  = 5
	shutdownTimeout = 5 * time.Second
)

type SnapshotSource interface {
	Snapshot() core.Snapshot
}

type Server struct {
	address         string
	source          SnapshotSource
	handler         http.Handler
	listen          func(network, address string) (net.Listener, error)
	shutdownTimeout time.Duration
}

func New(address string, source SnapshotSource) (*Server, error) {
	if source == nil {
		return nil, fmt.Errorf("snapshot source is required")
	}
	page, err := template.New("status").Parse(pageHTML)
	if err != nil {
		return nil, fmt.Errorf("parse status page template: %w", err)
	}
	server := &Server{
		address:         address,
		source:          source,
		listen:          net.Listen,
		shutdownTimeout: shutdownTimeout,
	}
	server.handler = server.newHandler(page)
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := s.listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.address, err)
	}

	httpServer := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

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

func (s *Server) newHandler(page *template.Template) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body bytes.Buffer
		if err := page.Execute(&body, s.source.Snapshot()); err != nil {
			http.Error(response, "render status page", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		response.WriteHeader(http.StatusOK)
		if request.Method == http.MethodGet {
			_, _ = response.Write(body.Bytes())
		}
	})
}

const pageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="5">
  <title>Soil Monitor</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, sans-serif; }
    body { margin: 0; background: #18221a; color: #f4f7f2; }
    main { width: min(48rem, calc(100% - 2rem)); margin: 3rem auto; }
    header { display: flex; justify-content: space-between; align-items: baseline; gap: 1rem; }
    h1 { margin: 0; font-size: clamp(2rem, 8vw, 4rem); }
    .status { padding: .4rem .7rem; border-radius: 999px; font-weight: 700; }
    .online { background: #2c7a45; }
    .offline { background: #8d3030; }
    .plants { display: grid; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); gap: 1rem; margin-top: 2rem; }
    .plant { padding: 1.25rem; border: 1px solid #526456; border-radius: .8rem; background: #223126; }
    .plant.selected { border-color: #9ed59f; }
    .plant h2 { margin: 0; font-size: 1.2rem; }
    .moisture { margin: .5rem 0; font-size: 3rem; font-variant-numeric: tabular-nums; }
    .id { margin: 0; color: #b8c8ba; font-size: .85rem; }
  </style>
</head>
<body>
  <main>
    <header>
      <h1>Soil Monitor</h1>
      {{if .Connected}}<p class="status online">MQTT ONLINE</p>{{else}}<p class="status offline">MQTT OFFLINE</p>{{end}}
    </header>
    <section class="plants" aria-label="Plant moisture readings">
      {{range .Plants}}
      <article class="plant{{if eq $.SelectedPlantID .ID}} selected{{end}}">
        <h2>{{.Name}}</h2>
        <p class="moisture">{{if .Moisture}}{{.Moisture}}%{{else}}--{{end}}</p>
        <p class="id">{{.ID}}</p>
      </article>
      {{end}}
    </section>
  </main>
</body>
</html>`
