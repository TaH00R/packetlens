package realtime

import (
	"testing"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func TestSlowClientsAndShutdown(t *testing.T) {
	h := NewHub()
	slow, fast := h.Register(), h.Register()
	for i := 0; i <= cap(slow.Send); i++ {
		h.Publish(monitor.Snapshot{})
		if _, ok := <-fast.Send; !ok {
			t.Fatal("healthy client disconnected")
		}
	}
	if h.ClientCount() != 1 {
		t.Fatalf("clients = %d", h.ClientCount())
	}
	if slow.Context.Err() == nil {
		t.Fatal("slow client's I/O was not cancelled")
	}
	for range slow.Send {
	}
	h.Unregister(slow)
	h.Close()
	if fast.Context.Err() == nil {
		t.Fatal("shutdown did not cancel client I/O")
	}
	h.Close()
	h.Publish(monitor.Snapshot{})
	if _, ok := <-fast.Send; ok {
		t.Fatal("shutdown left client open")
	}
	if _, ok := <-h.Register().Send; ok {
		t.Fatal("registered after shutdown")
	}
}
