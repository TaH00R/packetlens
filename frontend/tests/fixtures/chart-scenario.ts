import { report } from "./analytics";
import type { TimeRange } from "../../lib/monitoring";
// A short deterministic observed run, preceded by empty buckets. Only tests use it.
export function chartScenario(range: TimeRange) {
  const result = report(range);
  const values = [20, 32, 25, 40, 35];
  const sorted = [...values].sort((a, b) => a - b);
  const points = result.current.trends[0].points.map((point, index) =>
    index < 55
      ? point
      : {
          ...point,
          attempts: 1,
          successful: 1,
          avg_latency_ms: values[index - 55],
          availability: 100,
          jitter_ms:
            index === 55
              ? null
              : Math.abs(values[index - 55] - values[index - 56]),
        },
  );
  const source = result.current.targets[0];
  const summary = {
    ...source,
    attempts: 5,
    successful: 5,
    avg_latency_ms: 30.4,
    min_latency_ms: 20,
    max_latency_ms: 40,
    p50_ms: sorted[2],
    p90_ms: 40,
    p95_ms: 40,
    p99_ms: 40,
    jitter_ms: 9.75,
    current_jitter_ms: 5,
    first_observed: points[55].timestamp,
    last_observed: points[59].timestamp,
    histogram: source.histogram.map((bin) => ({
      ...bin,
      count: values.filter(
        (value) =>
          value >= bin.lower_ms &&
          (bin.upper_ms === null || value < bin.upper_ms),
      ).length,
    })),
  };
  result.current.targets = [summary];
  result.current.groups[0] = {
    ...summary,
    target: undefined,
    current_jitter_ms: null,
  };
  result.current.trends[0].points = points;
  result.current.target_trends = [
    { type: "tcp", target: source.target, points },
  ];
  return result;
}
