CREATE TABLE correlations (
    id TEXT PRIMARY KEY NOT NULL,
    cycle_timestamp TEXT NOT NULL,
    recovery_cycle TEXT,
    document TEXT NOT NULL
);
CREATE INDEX correlations_cycle ON correlations(cycle_timestamp,id);
CREATE INDEX correlations_ended ON correlations(recovery_cycle);
CREATE UNIQUE INDEX correlations_one_active ON correlations((1)) WHERE recovery_cycle IS NULL;
CREATE TABLE correlation_members (
    event_id TEXT NOT NULL REFERENCES correlations(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN ('tcp','dns')),
    target TEXT NOT NULL,
    PRIMARY KEY(event_id,type,target)
);
CREATE INDEX correlation_members_target ON correlation_members(type,target,event_id);
CREATE INDEX correlation_members_name ON correlation_members(target,event_id);
CREATE TABLE correlation_state (
    singleton INTEGER PRIMARY KEY CHECK(singleton=1),
    document TEXT NOT NULL
);
