package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
)

func TestWebSocketStreamAndShutdown(t *testing.T) {
	hub := realtime.NewHub()
	server := httptest.NewServer(NewWebSocketHandler(hub))
	defer server.Close()
	defer hub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"http://localhost:3000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	// The handshake can finish before the server has registered its queue.
	for hub.ClientCount() == 0 {
		select {
		case <-ctx.Done():
			t.Fatal("client was not registered")
		case <-time.After(time.Millisecond):
		}
	}
	want := monitor.Snapshot{Timestamp: time.Now().UTC(), TCP: []monitor.ProbeResult{}, DNS: []monitor.ProbeResult{}}
	hub.Publish(want)
	var got monitor.Snapshot
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Timestamp.Equal(want.Timestamp) || got.TCP == nil || got.DNS == nil {
		t.Fatalf("invalid streamed snapshot: %+v", got)
	}
	hub.Close()
	if err := wsjson.Read(ctx, conn, &got); err == nil {
		t.Fatal("socket stayed open after hub shutdown")
	}

	if forbidden, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"https://untrusted.example"}},
	}); err == nil {
		forbidden.CloseNow()
		t.Fatal("untrusted WebSocket origin allowed")
	} else if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected origin rejection, got response=%v error=%v", response, err)
	}
}
