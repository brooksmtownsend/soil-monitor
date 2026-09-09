package webdisplay

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"soilmonitor/internal/core"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
)

// wsPlant is the JSON representation of a plant sent over WebSocket.
type wsPlant struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Cadence           string `json:"cadence,omitempty"`
	NextWateringDate  string `json:"nextWateringDate,omitempty"`
	HasPhoto          bool   `json:"hasPhoto"`
	Percentage        *int   `json:"percentage,omitempty"`
	Raw               *int   `json:"raw,omitempty"`
	ObservedAt        string `json:"observedAt,omitempty"`
	ObservedAtDisplay string `json:"observedAtDisplay,omitempty"`
}

type wsSnapshot struct {
	Connected bool      `json:"connected"`
	Plants    []wsPlant `json:"plants"`
}

func marshalSnapshot(snap core.Snapshot) ([]byte, error) {
	plants := make([]wsPlant, len(snap.Plants))
	for i, p := range snap.Plants {
		wp := wsPlant{ID: p.ID, Name: p.Name, Cadence: string(p.Cadence), NextWateringDate: string(p.NextWateringDate), HasPhoto: p.HasPhoto}
		if p.Latest != nil {
			pct := p.Latest.Percentage
			raw := p.Latest.Raw
			wp.Percentage = &pct
			wp.Raw = &raw
			wp.ObservedAt = formatMachineTime(p.Latest.ObservedAt)
			wp.ObservedAtDisplay = formatEasternTime(p.Latest.ObservedAt)
		}
		plants[i] = wp
	}
	return json.Marshal(wsSnapshot{Connected: snap.Connected, Plants: plants})
}

type hubRegistration struct {
	client  *wsClient
	initial []byte
}

type hub struct {
	clients    map[*wsClient]struct{}
	register   chan hubRegistration
	unregister chan *wsClient
	broadcast  chan []byte
}

func newHub() *hub {
	return &hub{
		clients:    make(map[*wsClient]struct{}),
		register:   make(chan hubRegistration, 16),
		unregister: make(chan *wsClient, 16),
		broadcast:  make(chan []byte, 1),
	}
}

func (h *hub) run(done <-chan struct{}) {
	for {
		select {
		case <-done:
			for c := range h.clients {
				close(c.send)
			}
			return
		case reg := <-h.register:
			h.clients[reg.client] = struct{}{}
			if reg.initial != nil {
				select {
				case reg.client.send <- reg.initial:
				default:
				}
			}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					delete(h.clients, c)
					close(c.send)
				}
			}
		}
	}
}

type wsClient struct {
	hub  *hub
	conn *websocket.Conn
	send chan []byte
}

func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
