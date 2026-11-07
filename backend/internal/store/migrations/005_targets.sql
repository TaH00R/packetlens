CREATE TABLE monitoring_targets (
    id TEXT PRIMARY KEY NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('tcp','dns')),
    address TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
    interval_ms INTEGER NOT NULL CHECK(interval_ms BETWEEN 5000 AND 3600000),
    timeout_ms INTEGER NOT NULL CHECK(timeout_ms BETWEEN 100 AND 30000 AND timeout_ms <= interval_ms),
    UNIQUE(type,address)
);
-- Seed exactly once at migration, including when upgrading an existing database.
-- Deleting all targets later must not restore defaults on restart.
INSERT INTO monitoring_targets VALUES
('00000000000000000000000000000001','tcp','1.1.1.1:443',1,5000,3000),
('00000000000000000000000000000002','tcp','8.8.8.8:443',1,5000,3000),
('00000000000000000000000000000003','dns','google.com',1,5000,3000),
('00000000000000000000000000000004','dns','cloudflare.com',1,5000,3000);
