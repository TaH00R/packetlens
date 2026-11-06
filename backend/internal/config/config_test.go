package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	load := func(values map[string]string) (Config, error) {
		return Load(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	}
	cfg, err := load(nil)
	if err != nil || cfg.DatabasePath != "data/packetlens.db" || cfg.Retention != 30*24*time.Hour || cfg.CleanupInterval != time.Hour {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	cfg, err = load(map[string]string{"PACKETLENS_DB_PATH": "custom/history.db", "PACKETLENS_RETENTION": "24h", "PACKETLENS_CLEANUP_INTERVAL": "5m"})
	if err != nil || cfg.Retention != 24*time.Hour || cfg.CleanupInterval != 5*time.Minute {
		t.Fatalf("custom: %+v %v", cfg, err)
	}
	for _, values := range []map[string]string{
		{"PACKETLENS_ANOMALY_WINDOW": "2"}, {"PACKETLENS_ANOMALY_MIN_SAMPLES": "bad"}, {"PACKETLENS_ANOMALY_MAD_MULTIPLIER": "NaN"},
		{"PACKETLENS_ANOMALY_MIN_DELTA_MS": "Infinity"}, {"PACKETLENS_ANOMALY_MAX_AGE": "30s"}, {"PACKETLENS_ANOMALY_RECOVERY_SAMPLES": "0"},
		{"PACKETLENS_DB_PATH": " "}, {"PACKETLENS_RETENTION": ""}, {"PACKETLENS_RETENTION": "30d"}, {"PACKETLENS_RETENTION": "-1h"},
		{"PACKETLENS_RETENTION": "100000h"}, {"PACKETLENS_CLEANUP_INTERVAL": "0s"}, {"PACKETLENS_CLEANUP_INTERVAL": "25h"},
		{"PACKETLENS_RETENTION": "1h", "PACKETLENS_CLEANUP_INTERVAL": "2h"},
	} {
		if _, err := load(values); err == nil {
			t.Fatalf("accepted invalid configuration: %v", values)
		}
	}
}
