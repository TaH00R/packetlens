package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/incident"
)

type Config struct {
	DatabasePath    string
	Retention       time.Duration
	CleanupInterval time.Duration
	Anomaly         anomaly.Config
	Incident        incident.Config
}

// Load accepts an environment lookup so validation can be tested without
// mutating process-wide environment variables.
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if value, exists := lookup(key); exists {
			return strings.TrimSpace(value)
		}
		return fallback
	}
	c := Config{DatabasePath: get("PACKETLENS_DB_PATH", "data/packetlens.db")}
	if c.DatabasePath == "" {
		return c, fmt.Errorf("PACKETLENS_DB_PATH must not be empty")
	}
	var err error
	c.Retention, err = time.ParseDuration(get("PACKETLENS_RETENTION", "720h"))
	if err != nil || c.Retention < time.Hour || c.Retention > 10*365*24*time.Hour {
		return c, fmt.Errorf("PACKETLENS_RETENTION must be a duration between 1h and 87600h")
	}
	c.CleanupInterval, err = time.ParseDuration(get("PACKETLENS_CLEANUP_INTERVAL", "1h"))
	if err != nil || c.CleanupInterval < time.Minute || c.CleanupInterval > 24*time.Hour || c.CleanupInterval > c.Retention {
		return c, fmt.Errorf("PACKETLENS_CLEANUP_INTERVAL must be between 1m and 24h and no greater than retention")
	}
	c.Anomaly = anomaly.DefaultConfig()
	for _, field := range []struct {
		key         string
		destination *int
	}{{"WINDOW", &c.Anomaly.Window}, {"MIN_SAMPLES", &c.Anomaly.MinSamples}, {"RECOVERY_SAMPLES", &c.Anomaly.RecoverySamples}} {
		key := "PACKETLENS_ANOMALY_" + field.key
		if value, ok := lookup(key); ok {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return c, fmt.Errorf("%s must be an integer", key)
			}
			*field.destination = n
		}
	}
	for _, field := range []struct {
		key         string
		destination *float64
	}{{"MAD_MULTIPLIER", &c.Anomaly.Multiplier}, {"MIN_DELTA_MS", &c.Anomaly.MinDeltaMs}} {
		key := "PACKETLENS_ANOMALY_" + field.key
		if value, ok := lookup(key); ok {
			n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				return c, fmt.Errorf("%s must be numeric", key)
			}
			*field.destination = n
		}
	}
	if value, ok := lookup("PACKETLENS_ANOMALY_MAX_AGE"); ok {
		duration, err := time.ParseDuration(strings.TrimSpace(value))
		if err != nil {
			return c, fmt.Errorf("PACKETLENS_ANOMALY_MAX_AGE must be a duration")
		}
		c.Anomaly.MaxAge = duration
	}
	if err := c.Anomaly.Validate(); err != nil {
		return c, err
	}
	c.Incident = incident.DefaultConfig()
	for _, field := range []struct {
		key         string
		destination *int
	}{{"FAILURE_THRESHOLD", &c.Incident.FailureThreshold}, {"RECOVERY_THRESHOLD", &c.Incident.RecoveryThreshold}} {
		key := "PACKETLENS_INCIDENT_" + field.key
		if value, ok := lookup(key); ok {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return c, fmt.Errorf("%s must be an integer", key)
			}
			*field.destination = n
		}
	}
	if value, ok := lookup("PACKETLENS_INCIDENT_MAX_GAP"); ok {
		gap, err := time.ParseDuration(strings.TrimSpace(value))
		if err != nil {
			return c, fmt.Errorf("PACKETLENS_INCIDENT_MAX_GAP must be a duration")
		}
		c.Incident.MaxGap = gap
	}
	if err := c.Incident.Validate(); err != nil {
		return c, err
	}
	return c, nil
}
