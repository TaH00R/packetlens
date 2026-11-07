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
	if cfg.Incident.FailureThreshold != 3 || cfg.Incident.RecoveryThreshold != 2 || cfg.Incident.MaxGap != 30*time.Second {
		t.Fatalf("incident defaults: %+v", cfg.Incident)
	}
	if cfg.Correlation.Window != 15*time.Second || cfg.Correlation.MinTargets != 2 {
		t.Fatalf("correlation defaults: %+v", cfg.Correlation)
	}
	cfg, err = load(map[string]string{"PACKETLENS_DB_PATH": "custom/history.db", "PACKETLENS_RETENTION": "24h", "PACKETLENS_CLEANUP_INTERVAL": "5m"})
	if err != nil || cfg.Retention != 24*time.Hour || cfg.CleanupInterval != 5*time.Minute {
		t.Fatalf("custom: %+v %v", cfg, err)
	}
	for _, values := range []map[string]string{
		{"PACKETLENS_ADMIN_TOKEN": "short"}, {"PACKETLENS_ADMIN_TOKEN": "long enough token with embedded spaces is invalid"},
		{"PACKETLENS_CORRELATION_WINDOW": "bad"}, {"PACKETLENS_CORRELATION_WINDOW": "0s"}, {"PACKETLENS_CORRELATION_WINDOW": "2h"}, {"PACKETLENS_CORRELATION_MIN_TARGETS": "bad"}, {"PACKETLENS_CORRELATION_MIN_TARGETS": "1"}, {"PACKETLENS_CORRELATION_MIN_TARGETS": "129"},
		{"PACKETLENS_INCIDENT_FAILURE_THRESHOLD": "0"}, {"PACKETLENS_INCIDENT_FAILURE_THRESHOLD": "1001"}, {"PACKETLENS_INCIDENT_RECOVERY_THRESHOLD": "bad"},
		{"PACKETLENS_INCIDENT_RECOVERY_THRESHOLD": "0"}, {"PACKETLENS_INCIDENT_MAX_GAP": "0s"}, {"PACKETLENS_INCIDENT_MAX_GAP": "25h"},
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
