import type { AnomalyReport } from "../types/anomalies";
import { isRecord, isNonnegative, isCount, isTimestamp } from "./validation";

function policy(v: unknown): boolean {
  return isRecord(v) && isCount(v.window) && v.window >= 3 && v.window <= 500 && isCount(v.min_samples) && v.min_samples >= 3 && v.min_samples <= v.window &&
    isNonnegative(v.mad_multiplier) && v.mad_multiplier >= 1 && v.mad_multiplier <= 100 && isNonnegative(v.min_delta_ms) && v.min_delta_ms >= .1 && v.min_delta_ms <= 60000 &&
    isCount(v.max_age_ns) && v.max_age_ns >= 60e9 && v.max_age_ns <= 86400e9 && isCount(v.recovery_samples) && v.recovery_samples >= 1 && v.recovery_samples <= 100;
}
function event(v: unknown): boolean {
  if (!isRecord(v) || typeof v.id !== "string" || !/^[a-f0-9]{32}-[0-9]{1,6}$/.test(v.id) || (v.type !== "tcp" && v.type !== "dns") || typeof v.target !== "string" || v.target.length === 0 || v.target.length > 512) return false;
  if (![v.timestamp,v.cycle_timestamp,v.last_anomaly_at,v.last_observed_at].every(isTimestamp) || (v.recovered_at !== null && !isTimestamp(v.recovered_at)) || (v.recovery_cycle !== null && !isTimestamp(v.recovery_cycle)) || (v.recovered_at === null) !== (v.recovery_cycle === null)) return false;
  if (![v.observed_latency_ms,v.peak_latency_ms,v.baseline_ms,v.mad_ms,v.threshold_ms].every(isNonnegative)) return false;
  if ((v.observed_latency_ms as number) <= (v.threshold_ms as number) || (v.threshold_ms as number) <= (v.baseline_ms as number) || (v.peak_latency_ms as number) < (v.observed_latency_ms as number)) return false;
  if (Date.parse(String(v.last_anomaly_at)) < Date.parse(String(v.timestamp)) || Date.parse(String(v.last_observed_at)) < Date.parse(String(v.last_anomaly_at)) || (v.recovered_at !== null && Date.parse(String(v.recovered_at)) < Date.parse(String(v.last_observed_at)))) return false;
  return (v.severity === "warning" || v.severity === "critical") && v.method === "rolling-median-mad-v1" && policy(v.policy) && isRecord(v.policy) &&
    isCount(v.anomalous_samples) && v.anomalous_samples > 0 && isCount(v.baseline_samples) && v.baseline_samples >= Number(v.policy.min_samples) && v.baseline_samples <= Number(v.policy.window);
}
export function parseAnomalies(value: unknown): AnomalyReport {
  if (!isRecord(value) || !["5m","15m","1h","6h","24h","all"].includes(String(value.range)) || !isTimestamp(value.as_of) || !isCount(value.count) ||
      !Array.isArray(value.anomalies) || value.anomalies.length > 200 || value.count !== value.anomalies.length || !value.anomalies.every(event) ||
      typeof value.next_cursor !== "string" || value.next_cursor.length > 256 || typeof value.has_more !== "boolean" || value.has_more !== (value.next_cursor.length > 0)) throw new Error("Invalid anomaly response");
  return value as unknown as AnomalyReport;
}
