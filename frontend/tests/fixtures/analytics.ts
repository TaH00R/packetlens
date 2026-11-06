import type { AnalyticsPoint, AnalyticsReport, AnalyticsSummary, AvailabilityReport, ProbeType } from "../../types/analytics";
import type { TimeRange } from "../../lib/monitoring";

const at = Date.parse("2026-11-06T12:00:00Z");
const iso = (time: number) => new Date(time).toISOString();
const durations = { "5m": 300_000, "15m": 900_000, "1h": 3_600_000, "6h": 21_600_000, "24h": 86_400_000, all: 30*86_400_000 };

function summary(type: ProbeType, values: number[], target?: string): AnalyticsSummary {
  const sorted = [...values].sort((a,b) => a-b);
  const edges = [0,10,25,50,100,250,500,1000];
  return { type, ...(target ? { target } : {}), attempts: values.length, successful: values.length, failed: 0, availability: values.length ? 100 : null,
    avg_latency_ms: values.length ? values.reduce((a,b) => a+b,0)/values.length : null, min_latency_ms: sorted[0] ?? null, max_latency_ms: sorted.at(-1) ?? null,
    p50_ms: sorted[Math.ceil(values.length*.5)-1] ?? null, p90_ms: sorted.at(-1) ?? null, p95_ms: sorted.at(-1) ?? null, p99_ms: sorted.at(-1) ?? null,
    jitter_ms: null, current_jitter_ms: null, first_observed: values.length ? iso(at) : null, last_observed: values.length ? iso(at) : null,
    histogram: edges.map((lower_ms,i) => ({ lower_ms, upper_ms: edges[i+1] ?? null, count: values.filter((v) => v >= lower_ms && (edges[i+1] === undefined || v < edges[i+1])).length })) };
}
function points(from: number, successful = 0, average: number | null = null): AnalyticsPoint[] {
  return Array.from({ length: 60 }, (_, i) => ({ timestamp: iso(from+(at-from)*i/60), attempts: i === 59 ? successful : 0, successful: i === 59 ? successful : 0, avg_latency_ms: i === 59 ? average : null, jitter_ms: null, availability: i === 59 && successful > 0 ? 100 : null }));
}
export function report(range: TimeRange = "15m", mean = 50, empty = false): AnalyticsReport {
  const from = at-durations[range];
  const groups = [summary("tcp", empty ? [] : [mean-40,mean+40]), summary("dns", [])];
  const targets = empty ? [] : [summary("tcp", [mean-40], "alpha:443"), summary("tcp", [mean+40], "beta:443")];
  const current = { from: iso(from), to: iso(at), groups, targets,
    trends: groups.map((g) => ({ type: g.type, points: points(from,g.successful,g.avg_latency_ms) })),
    target_trends: targets.map((g) => ({ type: g.type, target: g.target, points: points(from,g.successful,g.avg_latency_ms) })) };
  return { range, as_of: iso(at), history_start: iso(from), current,
    previous: range === "all" ? null : { ...current, from: iso(from-durations[range]), to: iso(from-1), trends: [], target_trends: [] } };
}
export function availability(empty = false): AvailabilityReport {
  return { as_of: iso(at), history_start: iso(at-3_600_000), targets: [], groups: (["tcp","dns"] as const).map((type) => ({ type, consecutive_successes: 0, consecutive_failures: 0, last_success: null, last_failure: null,
    windows: (["1h","24h","7d","30d"] as const).map((window) => {
      const span = { "1h": 3_600_000, "24h": 86_400_000, "7d": 7*86_400_000, "30d": 30*86_400_000 }[window];
      const count = type === "tcp" && !empty ? 2 : 0;
      return { window, from: iso(at-span), to: iso(at), attempts: count, successful: count, availability: count ? 100 : null,
        first_observed: count ? iso(at) : null, last_observed: count ? iso(at) : null, trend: points(at-span,count,count ? 50 : null) };
    }) })) };
}
