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
	"github.com/TaH00R/packetlens/backend/internal/config"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
	"github.com/TaH00R/packetlens/backend/internal/target"
)

func main() {
	if err := run(); err != nil {
		log.Printf("PacketLens stopped with error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	startupCtx, startupCancel := context.WithTimeout(ctx, 15*time.Second)
	database, err := store.OpenSQLiteWithCorrelation(startupCtx, cfg.DatabasePath, cfg.Anomaly, cfg.Incident, cfg.Correlation)
	startupCancel()
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Database close failed: %v", err)
		}
	}()

	log.Printf("Failure correlation | window=%s | minimum_distinct_targets=%d", cfg.Correlation.Window, cfg.Correlation.MinTargets)
	hub := realtime.NewHub()
	log.Printf("History database: %s | retention=%s | cleanup=%s", cfg.DatabasePath, cfg.Retention, cfg.CleanupInterval)
	log.Printf("Failure incident detector | failure_threshold=%d | recovery_threshold=%d | maximum_observation_gap=%s", cfg.Incident.FailureThreshold, cfg.Incident.RecoveryThreshold, cfg.Incident.MaxGap)
	log.Printf("Latency anomaly detector | window=%d | minimum_samples=%d | mad_multiplier=%g | minimum_delta_ms=%g | baseline_max_age=%s | recovery_successes=%d", cfg.Anomaly.Window, cfg.Anomaly.MinSamples, cfg.Anomaly.Multiplier, cfg.Anomaly.MinDeltaMs, cfg.Anomaly.MaxAge, cfg.Anomaly.RecoverySamples)

	m := monitor.NewConfiguredMonitor(nil, nil, cfg.DNSResolvers...)
	targetCtx, targetCancel := context.WithTimeout(ctx, 5*time.Second)
	manager, err := target.NewManager(targetCtx, database, m.Reconfigure)
	targetCancel()
	if err != nil {
		return err
	}
	log.Printf("DNS resolver comparison configured | resolvers=%v", cfg.DNSResolvers)
	log.Printf("Target configuration loaded | count=%d | management_enabled=%t", len(manager.List()), cfg.AdminToken != "")

	var background sync.WaitGroup
	background.Add(2)

	go func() {
		defer background.Done()
		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()
		for {
			cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 30*time.Second)
			n, err := store.Cleanup(cleanupCtx, database, time.Now().Add(-cfg.Retention))
			cleanupCancel()
			if err != nil && ctx.Err() == nil {
				log.Printf("History cleanup failed: %v", err)
			}
			if n > 0 {
				log.Printf("History cleanup removed %d expired snapshots", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	go func() {
		defer background.Done()

		for snapshot := range m.Results {
			// Cancel pending writes on shutdown; committed cycles remain durable.
			writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
			saved, err := database.SaveSnapshot(writeCtx, snapshot)
			writeCancel()
			if err != nil {
				if ctx.Err() != nil {
					continue
				}
				log.Printf("Snapshot persistence failed; continuing live monitoring | timestamp=%s | error=%v", snapshot.Timestamp.Format(time.RFC3339Nano), err)
			} else {
				snapshot = saved
			}
			hub.Publish(snapshot)

			log.Printf(
				"[SNAPSHOT] TCP: %d | DNS: %d | Time: %s",
				len(snapshot.TCP),
				len(snapshot.DNS),
				snapshot.Timestamp.Format(time.RFC3339),
			)
		}
	}()

	go m.Start(ctx)

	server := api.NewServer(database, hub, &api.TargetAdmin{Manager: manager, Token: cfg.AdminToken, DNSResolvers: cfg.DNSResolvers})
	serverErrors := make(chan error, 1)

	go func() {
		log.Println("PacketLens API listening on :8080")

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			serverErrors <- err
			cancel()
		}
	}()

	<-ctx.Done()

	log.Println("Shutting down PacketLens...")
	hub.Close()

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	background.Wait()

	log.Println("PacketLens stopped.")
	select {
	case err := <-serverErrors:
		return err
	default:
		return nil
	}
}
