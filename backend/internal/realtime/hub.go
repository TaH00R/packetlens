package realtime

import (
	"context"
	"log"
	"sync"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type Client struct {
	Send    chan monitor.Snapshot
	Context context.Context
	cancel  context.CancelFunc
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
	closed  bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

func (h *Hub) Register() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		Send:    make(chan monitor.Snapshot, 64),
		Context: ctx,
		cancel:  cancel,
	}

	h.mu.Lock()
	if h.closed {
		client.cancel()
		close(client.Send)
	} else {
		h.clients[client] = true
	}
	h.mu.Unlock()

	return client
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.clients[client]; exists {
		client.cancel()
		delete(h.clients, client)
		close(client.Send)
	}
}

func (h *Hub) Publish(snapshot monitor.Snapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		select {
		case client.Send <- snapshot:
		default:
			log.Printf("[WS] Disconnecting slow client: snapshot queue is full")
			client.cancel()
			delete(h.clients, client)
			close(client.Send)
		}
	}
}

// Close releases streaming handlers, which HTTP Server.Shutdown does not close.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for client := range h.clients {
		client.cancel()
		delete(h.clients, client)
		close(client.Send)
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}
