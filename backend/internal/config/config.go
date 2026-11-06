package config

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	DatabasePath    string
	Retention       time.Duration
	CleanupInterval time.Duration
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
	return c, nil
}
