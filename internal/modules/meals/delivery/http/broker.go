package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type OrderEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Broker struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	allowed map[string]struct{}
}

func NewBroker(allowedOrigins []string) *Broker {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return &Broker{clients: make(map[*websocket.Conn]struct{}), allowed: allowed}
}

func (b *Broker) Serve(c *gin.Context, claimedOrders any) {
	upgrader := websocket.Upgrader{Subprotocols: []string{"bearer"}, CheckOrigin: func(r *http.Request) bool {
		if _, allOrigins := b.allowed["*"]; allOrigins {
			return true
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		normalized := parsed.Scheme + "://" + parsed.Host
		_, ok := b.allowed[normalized]
		return ok
	}}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	b.mu.Lock()
	b.clients[conn] = struct{}{}
	// Register and write the initial snapshot under the same lock used by
	// Publish, preventing concurrent writes to the WebSocket connection.
	err = conn.WriteJSON(OrderEvent{Type: "CLAIMED_ORDERS", Data: claimedOrders})
	b.mu.Unlock()
	if err != nil {
		b.mu.Lock()
		delete(b.clients, conn)
		b.mu.Unlock()
		_ = conn.Close()
		return
	}
	defer func() {
		b.mu.Lock()
		delete(b.clients, conn)
		b.mu.Unlock()
		_ = conn.Close()
	}()
	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (b *Broker) Publish(event OrderEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn := range b.clients {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err = conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			_ = conn.Close()
			delete(b.clients, conn)
		}
	}
}
