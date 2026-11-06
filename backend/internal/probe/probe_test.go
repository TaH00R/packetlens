package probe

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestTCPLocalAndCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if result := CheckTCP(context.Background(), listener.Addr().String()); !result.Success {
		t.Fatalf("local connection: %+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := CheckTCP(ctx, listener.Addr().String()); result.Success || result.Error == "" {
		t.Fatalf("cancelled probe: %+v", result)
	}
	if result := CheckTCP(context.Background(), "invalid-address"); result.Success || result.Error == "" {
		t.Fatalf("invalid target: %+v", result)
	}
}

func TestDNSDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if result := CheckDNS(ctx, "timeout.invalid"); result.Success || result.Error == "" {
		t.Fatalf("expired DNS probe: %+v", result)
	}
	if result := CheckDNS(context.Background(), "127.0.0.1"); !result.Success {
		t.Fatalf("local literal lookup: %+v", result)
	}
}
