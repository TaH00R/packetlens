import type { IncidentReport } from "../types/incidents";
import { isRecord, isCount, isTimestamp, isNonnegative } from "./validation";

function incident(value: unknown): boolean {
  if (!isRecord(value) || typeof value.id !== "string" || !/^[a-f0-9]{32}-[0-9]{1,6}$/.test(value.id) || (value.type !== "tcp" && value.type !== "dns") || typeof value.target !== "string" || value.target.length === 0 || value.target.length > 512) return false;
  if (![value.started_at,value.cycle_timestamp,value.detected_at,value.detected_cycle,value.last_failure_at,value.last_observed_at].every(isTimestamp)) return false;
  const started=Date.parse(String(value.started_at)), detected=Date.parse(String(value.detected_at)), failed=Date.parse(String(value.last_failure_at)), observed=Date.parse(String(value.last_observed_at));
  if (detected < started || failed < detected || observed < failed) return false;
  const p=value.policy;
  if (!isRecord(p) || !isCount(p.failure_threshold) || p.failure_threshold<1 || p.failure_threshold>1000 || !isCount(p.recovery_threshold) || p.recovery_threshold<1 || p.recovery_threshold>100 || !isCount(p.max_gap_ns) || p.max_gap_ns<1e9 || p.max_gap_ns>86400e9) return false;
  if (!isCount(value.failed_probes) || value.failed_probes<p.failure_threshold || !isCount(value.recovery_successes) || value.recovery_successes>p.recovery_threshold || typeof value.has_monitoring_gaps!=="boolean") return false;
  if (typeof value.first_error!=="string" || [...value.first_error].length>1024 || typeof value.last_error!=="string" || [...value.last_error].length>1024) return false;
  if (value.recovered_at === null) return value.recovery_cycle===null && value.duration_ms===null && value.recovery_successes<p.recovery_threshold;
  return isTimestamp(value.recovered_at) && isTimestamp(value.recovery_cycle) && isNonnegative(value.duration_ms) && Date.parse(value.recovered_at)===observed && Math.abs(value.duration_ms-(observed-started))<=1 && value.recovery_successes===p.recovery_threshold;
}

export function parseIncidents(value: unknown): IncidentReport {
  if (!isRecord(value) || !["5m","15m","1h","6h","24h","all"].includes(String(value.range)) || !isTimestamp(value.as_of) || !["all","open","recovered"].includes(String(value.status)) ||
      !isCount(value.count) || !isCount(value.open_count) || !isCount(value.recovered_count) || !Array.isArray(value.incidents) || value.incidents.length>200 || value.count!==value.incidents.length || !value.incidents.every(incident) ||
      typeof value.has_more!=="boolean" || typeof value.next_cursor!=="string" || value.next_cursor.length>256 || value.has_more!==(value.next_cursor.length>0)) throw new Error("Invalid incident response");
  if (value.count>value.open_count+value.recovered_count || (value.status==="open" && value.incidents.some((e)=>e.recovered_at!==null)) || (value.status==="recovered" && value.incidents.some((e)=>e.recovered_at===null))) throw new Error("Invalid incident response");
  if (value.incidents.filter((e)=>e.recovered_at===null).length>value.open_count || value.incidents.filter((e)=>e.recovered_at!==null).length>value.recovered_count || new Set(value.incidents.map((e)=>e.id)).size!==value.count) throw new Error("Invalid incident response");
  return value as unknown as IncidentReport;
}

export function formatIncidentDuration(ms: number | null): string {
  if (ms===null) return "Awaiting recovery";
  const seconds=Math.floor(ms/1000);
  if (seconds<60) return `${seconds}s`;
  const minutes=Math.floor(seconds/60); if (minutes<60) return `${minutes}m ${seconds%60}s`;
  const hours=Math.floor(minutes/60); if (hours<24) return `${hours}h ${minutes%60}m`;
  return `${Math.floor(hours/24)}d ${hours%24}h`;
}
