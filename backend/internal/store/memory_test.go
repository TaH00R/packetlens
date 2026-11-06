package store

import (
	"sync"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func TestCapacityAndIsolation(t *testing.T) {
	s := NewMemoryStore(2)
	for i := 0; i < 3; i++ {
		s.AddSnapshot(monitor.Snapshot{Timestamp: time.Unix(int64(i), 0), TCP: []monitor.ProbeResult{{Target: "original"}}})
	}
	got := s.GetSnapshots()
	if len(got) != 2 || got[0].Timestamp.Unix() != 1 || got[1].Timestamp.Unix() != 2 {
		t.Fatalf("unexpected retention: %+v", got)
	}
	got[0].TCP[0].Target = "changed"
	if s.GetSnapshots()[0].TCP[0].Target != "original" {
		t.Fatal("read leaked mutable store slices")
	}
	input := monitor.Snapshot{TCP: []monitor.ProbeResult{{Target: "input"}}}
	s.AddSnapshot(input)
	input.TCP[0].Target = "changed"
	if s.GetSnapshots()[1].TCP[0].Target != "input" {
		t.Fatal("write retained mutable input slices")
	}
	results := s.GetAll()
	results[0].Target = "changed"
	if s.GetAll()[0].Target == "changed" {
		t.Fatal("flattened results alias store")
	}
	if NewMemoryStore(0).GetAll() == nil {
		t.Fatal("empty results must serialize as an array")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewMemoryStore(10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				s.AddSnapshot(monitor.Snapshot{})
				s.GetSnapshots()
				s.GetAll()
				s.Count()
			}
		})
	}
	wg.Wait()
	if s.Count() != 10 {
		t.Fatalf("count = %d", s.Count())
	}
}
