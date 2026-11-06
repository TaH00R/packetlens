import assert from "node:assert/strict";
import test from "node:test";
import { formatMs, formatRate, loadReport, parseAnalytics, parseAvailability, percentChange } from "../lib/analytics.ts";

const at = "2026-11-06T12:00:00Z";
const from = "2026-11-06T11:45:00Z";
const clone = (value) => structuredClone(value);
function points() { return Array.from({ length: 60 }, (_, i) => ({ timestamp: new Date(Date.parse(from)+i*15_000).toISOString(), attempts: 0, successful: 0, avg_latency_ms: null, jitter_ms: null, availability: null })) }
function summary(type) {
  const edges = [0, 10, 25, 50, 100, 250, 500, 1000];
  return { type, attempts: 0, successful: 0, failed: 0, availability: null, avg_latency_ms: null, min_latency_ms: null, max_latency_ms: null,
    p50_ms: null, p90_ms: null, p95_ms: null, p99_ms: null, jitter_ms: null, current_jitter_ms: null,
    first_observed: null, last_observed: null, histogram: edges.map((lower_ms, i) => ({ lower_ms, upper_ms: edges[i+1] ?? null, count: 0 })) };
}
function period(trends) {
  return { from, to: at, groups: [summary("tcp"), summary("dns")], targets: [], trends: trends ? ["tcp", "dns"].map((type) => ({ type, points: points() })) : [], target_trends: [] };
}
function analytics() { return { range: "15m", as_of: at, history_start: null, current: period(true), previous: period(false) } }
function available() {
  return { as_of: at, history_start: null, targets: [], groups: ["tcp", "dns"].map((type) => ({ type, consecutive_successes: 0, consecutive_failures: 0, last_success: null, last_failure: null,
    windows: ["1h", "24h", "7d", "30d"].map((window) => ({ window, from, to: at, attempts: 0, successful: 0, availability: null, first_observed: null, last_observed: null, trend: points() })) })) };
}

test("analytics validates nested reports, including empty and all-available ranges", () => {
  const valid = analytics();
  assert.deepEqual(parseAnalytics(valid), valid);
  assert.equal(parseAnalytics({ ...valid, range: "all", previous: null }).previous, null);
  for (const mutate of [
    (v) => { v.current.groups[0].p95_ms = "10" },
    (v) => { v.current.groups[0].p95_ms = NaN },
    (v) => { v.current.groups[0].availability = 100 },
    (v) => { v.current.groups[0].histogram[0].count = 1 },
    (v) => { v.current.groups[0].failed = -1 },
    (v) => { v.current.trends[0].points[0].avg_latency_ms = 0 },
    (v) => { v.current.trends[0].points[0].timestamp = "invalid" },
    (v) => { v.current.groups[1].type = "tcp" },
    (v) => { v.current.targets = [{ ...summary("tcp"), target: "" }] },
    (v) => { v.previous = null },
    (v) => { v.range = "7d" },
  ]) {
    const invalid = clone(valid); mutate(invalid);
    assert.throws(() => parseAnalytics(invalid), /Invalid analytics/);
  }
});

test("all-failed latency data remains null while observed availability is zero", () => {
  const report = analytics();
  const failed = report.current.groups[0];
  failed.attempts = 3; failed.failed = 3; failed.availability = 0;
  const parsed = parseAnalytics(report);
  assert.equal(parsed.current.groups[0].p50_ms, null);
  assert.equal(formatMs(parsed.current.groups[0].p95_ms), "No data");
  assert.equal(formatRate(parsed.current.groups[0].availability), "0.00%");
  assert.equal(formatMs(0), "0.00 ms");
  assert.equal(formatRate(null), "No observations");
});

test("availability validates windows, observations and streaks without filling gaps", () => {
  const report = available();
  assert.deepEqual(parseAvailability(report), report);
  assert.equal(parseAvailability(report).groups[0].windows[0].trend[0].availability, null);
  for (const mutate of [
    (v) => { v.groups[0].windows[0].window = "bad" },
    (v) => { v.groups[0].windows[0].availability = 101 },
    (v) => { v.groups[0].consecutive_successes = 1; v.groups[0].consecutive_failures = 1 },
    (v) => { v.groups[0].last_failure = "invalid" },
    (v) => { v.groups[0].windows[1].successful = 1 },
    (v) => { v.groups[0].windows.pop() },
  ]) { const invalid = clone(report); mutate(invalid); assert.throws(() => parseAvailability(invalid), /Invalid availability/); }
});

test("period comparison reports unavailable zero baselines without infinities", () => {
  assert.equal(percentChange(15, 10), "+50.0%");
  assert.equal(percentChange(5, 10), "-50.0%");
  assert.equal(percentChange(0, 0), "No comparable baseline");
  assert.equal(percentChange(null, 10), "No comparable baseline");
});

test("API loading validates payloads, forwards cancellation, and rejects errors", async () => {
  const controller = new AbortController();
  const valid = analytics();
  const fetcher = async (url, options) => {
    assert.equal(url, "http://local.test/api/analytics");
    assert.equal(options.signal, controller.signal);
    assert.equal(options.cache, "no-store");
    return new Response(JSON.stringify(valid), { status: 200 });
  };
  assert.deepEqual(await loadReport("http://local.test/api/analytics", parseAnalytics, controller.signal, fetcher), valid);
  await assert.rejects(loadReport("unused", parseAnalytics, controller.signal, async () => new Response("unavailable", { status: 503 })), /503/);
  await assert.rejects(loadReport("unused", parseAnalytics, controller.signal, async () => new Response("{}", { status: 200 })), /Invalid analytics/);
  await assert.rejects(loadReport("unused", parseAnalytics, controller.signal, async () => { throw new Error("offline") }), /offline/);
  controller.abort();
  await assert.rejects(loadReport("unused", parseAnalytics, controller.signal, async (_url, { signal }) => { signal.throwIfAborted() }), /abort/i);
});
