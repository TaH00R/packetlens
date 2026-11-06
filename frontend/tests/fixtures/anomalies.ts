import type { AnomalyReport, LatencyAnomaly } from "../../types/anomalies";

export function episode(recovered = false): LatencyAnomaly {
  return { id:"0123456789abcdef0123456789abcdef-0",type:"tcp",target:"alpha:443",
    timestamp:"2026-11-06T11:59:00Z",cycle_timestamp:"2026-11-06T11:59:00Z",last_anomaly_at:"2026-11-06T12:00:00Z",last_observed_at:"2026-11-06T12:01:00Z",
    recovered_at:recovered ? "2026-11-06T12:01:00Z" : null,recovery_cycle:recovered ? "2026-11-06T12:01:00Z" : null,
    observed_latency_ms:50,peak_latency_ms:100,baseline_ms:10,mad_ms:0,threshold_ms:20,severity:"critical",anomalous_samples:3,
    method:"rolling-median-mad-v1",policy:{window:60,min_samples:20,mad_multiplier:6,min_delta_ms:10,max_age_ns:900e9,recovery_samples:3},baseline_samples:20 };
}
export function anomalyReport(range="15m",events:LatencyAnomaly[]=[]):AnomalyReport {
  return {range,as_of:"2026-11-06T12:05:00Z",count:events.length,anomalies:events,has_more:false,next_cursor:""};
}
