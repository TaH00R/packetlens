CREATE TABLE latency_anomalies (
    id TEXT PRIMARY KEY NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('tcp','dns')),
    target TEXT NOT NULL,
    cycle_timestamp TEXT NOT NULL,
    recovery_cycle TEXT,
    document TEXT NOT NULL
);
CREATE INDEX latency_anomalies_cycle ON latency_anomalies(cycle_timestamp, id);
CREATE INDEX latency_anomalies_target ON latency_anomalies(type, target, cycle_timestamp, id);
CREATE UNIQUE INDEX latency_anomalies_open ON latency_anomalies(type, target) WHERE recovery_cycle IS NULL;
CREATE INDEX latency_anomalies_recovery ON latency_anomalies(recovery_cycle);

CREATE TABLE anomaly_state (
    type TEXT NOT NULL,
    target TEXT NOT NULL,
    last_observed TEXT NOT NULL,
    active_id TEXT REFERENCES latency_anomalies(id),
    document TEXT NOT NULL,
    PRIMARY KEY(type, target)
);
CREATE INDEX anomaly_state_observed ON anomaly_state(last_observed);
