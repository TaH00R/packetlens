export type ProbeType = "tcp" | "dns";

export interface HistogramBucket { lower_ms: number; upper_ms: number | null; count: number }
export interface AnalyticsSummary {
  type: ProbeType;
  target?: string;
  attempts: number;
  successful: number;
  failed: number;
  availability: number | null;
  avg_latency_ms: number | null;
  min_latency_ms: number | null;
  max_latency_ms: number | null;
  p50_ms: number | null;
  p90_ms: number | null;
  p95_ms: number | null;
  p99_ms: number | null;
  jitter_ms: number | null;
  current_jitter_ms: number | null;
  first_observed: string | null;
  last_observed: string | null;
  histogram: HistogramBucket[];
}
export interface AnalyticsPoint {
  anomaly_count?: number;
  anomaly_latency_ms?: number | null;
  timestamp: string;
  attempts: number;
  successful: number;
  avg_latency_ms: number | null;
  jitter_ms: number | null;
  availability: number | null;
}
export interface AnalyticsSeries { type: ProbeType; target?: string; points: AnalyticsPoint[] }
export interface AnalyticsPeriod {
  from: string;
  to: string;
  groups: AnalyticsSummary[];
  targets: AnalyticsSummary[];
  trends: AnalyticsSeries[];
  target_trends: AnalyticsSeries[];
}
export interface AnalyticsReport {
  range: string;
  as_of: string;
  history_start: string | null;
  current: AnalyticsPeriod;
  previous: AnalyticsPeriod | null;
}
export interface AvailabilityWindow {
  window: "1h" | "24h" | "7d" | "30d";
  from: string;
  to: string;
  attempts: number;
  successful: number;
  availability: number | null;
  first_observed: string | null;
  last_observed: string | null;
  trend: AnalyticsPoint[];
}
export interface AvailabilityTarget {
  type: ProbeType;
  target?: string;
  consecutive_successes: number;
  consecutive_failures: number;
  last_success: string | null;
  last_failure: string | null;
  windows: AvailabilityWindow[];
}
export interface AvailabilityReport {
  as_of: string;
  history_start: string | null;
  groups: AvailabilityTarget[];
  targets: AvailabilityTarget[];
}
export interface RemoteReport<T> { data: T | null; loading: boolean; error: string | null }
