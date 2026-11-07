import type { CorrelationReport, CorrelationEvent } from "../../types/correlations";
export function correlationEvent(ended = false): CorrelationEvent {
  return { id: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-0", cycle_timestamp: "2026-11-07T12:00:00Z", detected_at: "2026-11-07T12:00:00Z", last_correlated_at: "2026-11-07T12:00:05Z", ended_at: ended ? "2026-11-07T12:00:10Z" : null, end_cycle: ended ? "2026-11-07T12:00:10Z" : null, members: [{ type: "tcp", target: "a:443" }, { type: "dns", target: "example.test" }], policy: { window_ns: 15e9, min_targets: 2 } };
}
export function correlationReport(range = "15m", events: CorrelationEvent[] = []): CorrelationReport {
  return { range: range as CorrelationReport["range"], as_of: "2026-11-07T12:01:00Z", count: events.length, correlations: events, has_more: false, next_cursor: "" };
}
