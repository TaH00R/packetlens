import type { ResolverReport, ResolverSummary } from "../../types/resolvers";
export function resolverReport(range = "15m", withData = false): ResolverReport {
  const from = "2026-11-07T12:00:00Z", to = "2026-11-07T12:05:00Z";
  const series = (endpoint: string, failed: boolean): ResolverSummary => ({ resolver: endpoint, target: "", attempts: 1, successful: failed ? 0 : 1, failed: failed ? 1 : 0, timeouts: failed ? 1 : 0, avg_latency_ms: failed ? null : 20, min_latency_ms: failed ? null : 20, max_latency_ms: failed ? null : 20, last_failure_at: failed ? to : null, last_error: failed ? "DNS query timed out" : "", trend: Array.from({ length: 60 }, (_, i) => ({ timestamp: new Date(Date.parse(from) + i * 5000).toISOString(), attempts: i === 59 ? 1 : 0, successful: i === 59 && !failed ? 1 : 0, avg_latency_ms: i === 59 && !failed ? 20 : null, availability: i === 59 ? failed ? 0 : 100 : null })) });
  const groups = withData ? [series("127.0.0.1:53", false), series("127.0.0.2:53", true)] : [];
  return { range: range as ResolverReport["range"], as_of: to, from, to, configured_resolvers: withData ? ["127.0.0.1:53", "127.0.0.2:53"] : [], groups, targets: groups.map(s => ({ ...s, target: "example.test" })) };
}
