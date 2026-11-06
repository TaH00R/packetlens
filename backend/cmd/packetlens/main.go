package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/api"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	hub := realtime.NewHub()
	memoryStore := store.NewMemoryStore(1000)

	m := monitor.NewMonitor(
		5*time.Second,
		[]string{
			"1.1.1.1:443",
			"8.8.8.8:443",
		},
		[]string{
			"google.com",
			"cloudflare.com",
		},
	)

	var collector sync.WaitGroup
	collector.Add(1)

	go func() {
		defer collector.Done()

		for result := range m.Results {
			memoryStore.Add(result)
			hub.Publish(result)

			log.Printf("[PROBE] %+v", result)
		}
	}()

	go m.Start(ctx)

	server := api.NewServer(memoryStore, hub)

	go func() {
		log.Println("PacketLens API listening on :8080")

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()

	log.Println("Shutting down PacketLens...")

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	collector.Wait()

	log.Println("PacketLens stopped.")
}
