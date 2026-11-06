CREATE TABLE snapshots (
    id TEXT PRIMARY KEY NOT NULL,
    timestamp TEXT NOT NULL
);
CREATE INDEX snapshots_timestamp_id ON snapshots(timestamp, id);

CREATE TABLE probe_results (
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('tcp', 'dns')),
    target TEXT NOT NULL,
    latency_ms REAL NOT NULL CHECK(latency_ms >= 0),
    success INTEGER NOT NULL CHECK(success IN (0, 1)),
    error TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL,
    PRIMARY KEY(snapshot_id, ordinal)
);
CREATE INDEX probe_results_timestamp ON probe_results(timestamp);
CREATE INDEX probe_results_target_type ON probe_results(target, type, snapshot_id);
CREATE INDEX probe_results_type ON probe_results(type, snapshot_id);
