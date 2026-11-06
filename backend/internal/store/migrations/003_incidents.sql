CREATE TABLE incidents (
    id TEXT PRIMARY KEY NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('tcp','dns')),
    target TEXT NOT NULL,
    cycle_timestamp TEXT NOT NULL,
    recovery_cycle TEXT,
    document TEXT NOT NULL
);
CREATE INDEX incidents_cycle ON incidents(cycle_timestamp, id);
CREATE INDEX incidents_target ON incidents(type, target, cycle_timestamp, id);
CREATE UNIQUE INDEX incidents_open ON incidents(type, target) WHERE recovery_cycle IS NULL;
CREATE INDEX incidents_recovery ON incidents(recovery_cycle);

CREATE TABLE incident_state (
    type TEXT NOT NULL,
    target TEXT NOT NULL,
    last_observed TEXT NOT NULL,
    active_id TEXT REFERENCES incidents(id),
    document TEXT NOT NULL,
    PRIMARY KEY(type, target)
);
CREATE INDEX incident_state_observed ON incident_state(last_observed);
