CREATE TABLE dns_resolver_results (
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    target TEXT NOT NULL,
    resolver TEXT NOT NULL,
    latency_ms REAL NOT NULL CHECK(latency_ms >= 0),
    success INTEGER NOT NULL CHECK(success IN (0,1)),
    outcome TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL,
    PRIMARY KEY(snapshot_id,ordinal)
);
CREATE INDEX dns_resolver_timestamp ON dns_resolver_results(timestamp);
CREATE INDEX dns_resolver_target ON dns_resolver_results(target,resolver,snapshot_id);
CREATE INDEX dns_resolver_endpoint ON dns_resolver_results(resolver,snapshot_id);
