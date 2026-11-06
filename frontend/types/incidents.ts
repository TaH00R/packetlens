import type { ProbeType } from "./analytics";
export interface IncidentPolicy { failure_threshold: number; recovery_threshold: number; max_gap_ns: number }
export interface FailureIncident {
  id: string; type: ProbeType; target: string;
  started_at: string; cycle_timestamp: string; detected_at: string; detected_cycle: string;
  last_failure_at: string; last_observed_at: string;
  recovered_at: string | null; recovery_cycle: string | null; duration_ms: number | null;
  failed_probes: number; recovery_successes: number; has_monitoring_gaps: boolean;
  first_error: string; last_error: string; policy: IncidentPolicy;
}
export interface IncidentReport {
  range: string; as_of: string; status: "all" | "open" | "recovered";
  count: number; incidents: FailureIncident[]; open_count: number; recovered_count: number;
  has_more: boolean; next_cursor: string;
}
