package api

import (
	"context"
	"io"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/TaH00R/packetlens/backend/internal/realtime"
)

type WebSocketHandler struct {
	hub *realtime.Hub
}

func NewWebSocketHandler(hub *realtime.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

func (h *WebSocketHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"localhost:3000",
			"127.0.0.1:3000",
		},
	})

	if err != nil {
		return
	}

	defer conn.CloseNow()

	client := h.hub.Register()
	defer h.hub.Unregister(client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read incoming messages to detect client disconnection.
	go func() {
		defer cancel()

		for {
			_, reader, err := conn.Reader(ctx)
			if err != nil {
				return
			}

			if _, err := io.Copy(io.Discard, reader); err != nil {
				return
			}
		}
	}()

	// Stream monitoring results to the client.
	for {
		select {
		case <-ctx.Done():
			return

		case result, ok := <-client.Send:
			if !ok {
				return
			}

			if err := wsjson.Write(ctx, conn, result); err != nil {
				return
			}
		}
	}
}
