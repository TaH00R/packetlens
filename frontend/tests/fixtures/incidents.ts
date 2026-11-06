import type { FailureIncident, IncidentReport } from "../../types/incidents";

export function failureIncident(recovered=false): FailureIncident {
  return { id:"0123456789abcdef0123456789abcdef-0", type:"tcp", target:"alpha:443",
    started_at:"2026-11-06T11:59:00Z", cycle_timestamp:"2026-11-06T11:59:00Z", detected_at:"2026-11-06T11:59:10Z", detected_cycle:"2026-11-06T11:59:10Z",
    last_failure_at:"2026-11-06T12:00:00Z", last_observed_at:"2026-11-06T12:01:00Z",
    recovered_at:recovered?"2026-11-06T12:01:00Z":null, recovery_cycle:recovered?"2026-11-06T12:01:00Z":null, duration_ms:recovered?120000:null,
    failed_probes:5, recovery_successes:recovered?2:1, has_monitoring_gaps:true,
    first_error:"connection refused", last_error:"request timed out", policy:{failure_threshold:3,recovery_threshold:2,max_gap_ns:30e9} };
}
export function incidentReport(range="15m",events:FailureIncident[]=[],status:IncidentReport["status"]="all"):IncidentReport {
  return { range, as_of:"2026-11-06T12:05:00Z", status, count:events.length, incidents:events,
    open_count:events.filter((e)=>e.recovered_at===null).length, recovered_count:events.filter((e)=>e.recovered_at!==null).length,
    has_more:false,next_cursor:"" };
}
