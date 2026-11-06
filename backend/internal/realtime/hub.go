package realtime

import (
	"sync"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type Client struct {
	Send chan monitor.ProbeResult
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

func (h *Hub) Register() *Client {
	client := &Client{
		Send: make(chan monitor.ProbeResult, 64),
	}

	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	return client
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.clients[client]; exists {
		delete(h.clients, client)
		close(client.Send)
	}
}

func (h *Hub) Publish(result monitor.ProbeResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		select {
		case client.Send <- result:
		default:
			// Disconnect clients that cannot keep up.
			delete(h.clients, client)
			close(client.Send)
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}
