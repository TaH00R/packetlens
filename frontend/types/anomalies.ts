import type { ProbeType } from "./analytics";
export interface AnomalyPolicy {
  window: number; min_samples: number; mad_multiplier: number;
  min_delta_ms: number; max_age_ns: number; recovery_samples: number;
}
export interface LatencyAnomaly {
  id: string; type: ProbeType; target: string;
  timestamp: string; cycle_timestamp: string; last_anomaly_at: string; last_observed_at: string;
  recovered_at: string | null; recovery_cycle: string | null;
  observed_latency_ms: number; peak_latency_ms: number;
  baseline_ms: number; mad_ms: number; threshold_ms: number;
  severity: "warning" | "critical"; anomalous_samples: number;
  method: "rolling-median-mad-v1"; policy: AnomalyPolicy; baseline_samples: number;
}
export interface AnomalyReport {
  range: string; as_of: string; count: number; anomalies: LatencyAnomaly[];
  has_more: boolean; next_cursor: string;
}
