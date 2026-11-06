import assert from "node:assert/strict";
import test from "node:test";
import { filterHistory, isSnapshot, mergeHistory, parseHistory, HISTORY_CAPACITY } from "../lib/monitoring.ts";

const now = Date.parse("2026-11-06T10:00:00Z");
function snapshot(time = now, latency = 12) {
  const timestamp = new Date(time).toISOString();
  return { timestamp, tcp: [{ type: "tcp", target: "1.1.1.1:443", latency_ms: latency, success: true, timestamp }], dns: [] };
}

test("validates nested probe values, timestamps and history envelopes", () => {
  const valid = snapshot();
  assert.equal(isSnapshot(valid), true);
  for (const value of [null, {}, { ...valid, timestamp: "bad" }, { ...valid, tcp: [null] },
    { ...valid, tcp: [{ ...valid.tcp[0], latency_ms: NaN }] },
    { ...valid, tcp: [{ ...valid.tcp[0], latency_ms: -1 }] },
    { ...valid, tcp: [{ ...valid.tcp[0], type: "dns" }] },
    { ...valid, tcp: [{ ...valid.tcp[0], success: "true" }] }]) {
    assert.equal(isSnapshot(value), false);
  }
  assert.deepEqual(parseHistory({ count: 1, snapshots: [valid] }), [valid]);
  assert.throws(() => parseHistory({ count: 1, snapshots: [] }));
  assert.throws(() => parseHistory({ count: 1, snapshots: [null] }));
});

test("late REST history merges without losing newer live samples or timestamp collisions", () => {
  const old = snapshot(now - 1000);
  const live = snapshot();
  const collision = snapshot(now, 20);
  assert.deepEqual(mergeHistory([live], [old, live, collision]), [old, live, collision]);
  const multi = { ...live, tcp: [live.tcp[0], { ...live.tcp[0], target: "8.8.8.8:443" }] };
  assert.equal(mergeHistory([multi], [{ ...multi, tcp: [...multi.tcp].reverse() }]).length, 1);
});

test("every range includes its lower boundary, excludes future data and advances without new samples", () => {
  const ranges = { "5m": 300_000, "15m": 900_000, "1h": 3_600_000, "6h": 21_600_000, "24h": 86_400_000 };
  for (const [range, duration] of Object.entries(ranges)) {
    const boundary = snapshot(now - duration);
    const history = [snapshot(now - duration - 1), boundary, snapshot(), snapshot(now + 1)];
    assert.deepEqual(filterHistory(history, range, now), [boundary, history[2]]);
    assert.deepEqual(filterHistory([boundary], range, now + 1), []);
    assert.equal(history.length, 4);
  }
  const history = [snapshot(now - 100_000_000)];
  assert.equal(filterHistory(history, "all", now), history);
});

test("retains more than 100 live samples and caps memory at browser capacity", () => {
  const history = Array.from({ length: HISTORY_CAPACITY + 2 }, (_, i) => snapshot(now + i));
  const merged = mergeHistory(history);
  assert.equal(merged.length, HISTORY_CAPACITY);
  assert.deepEqual(merged[0], history[2]);
});

test("persistent snapshot IDs deduplicate REST and live data across reloads", () => {
  const id = "0123456789abcdef0123456789abcdef";
  const live = { ...snapshot(), id };
  const restored = JSON.parse(JSON.stringify(live));
  assert.equal(isSnapshot(restored), true);
  assert.equal(mergeHistory([live], [restored]).length, 1);
  assert.equal(mergeHistory([live], [{ ...live, id: "ffffffffffffffffffffffffffffffff" }]).length, 2);
  assert.equal(isSnapshot({ ...live, id: 123 }), false);
  assert.equal(isSnapshot({ ...live, id: "" }), false);
  assert.equal(isSnapshot({ ...live, id: "untrusted" }), false);
  // The legacy payload remains supported during a rolling backend upgrade.
  assert.equal(isSnapshot(snapshot()), true);
});
