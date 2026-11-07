import type { CorrelationReport } from "../types/correlations";
import { isRecord, isCount, isTimestamp } from "./validation";

function validEvent(value: unknown): boolean {
  if (!isRecord(value) || typeof value.id !== "string" || !/^[a-f0-9]{32}-[0-9]{1,6}$/.test(value.id) || !isTimestamp(value.cycle_timestamp) || !isTimestamp(value.detected_at) || !isTimestamp(value.last_correlated_at) || Date.parse(value.last_correlated_at) < Date.parse(value.detected_at)) return false;
  if (Date.parse(value.detected_at) < Date.parse(value.cycle_timestamp)) return false;
  if (value.ended_at === null ? value.end_cycle !== null : !isTimestamp(value.ended_at) || !isTimestamp(value.end_cycle) || Date.parse(value.ended_at) < Date.parse(value.last_correlated_at) || Date.parse(value.end_cycle) < Date.parse(value.cycle_timestamp) || Date.parse(value.end_cycle) > Date.parse(value.ended_at)) return false;
  const policy = value.policy;
  if (!isRecord(policy) || !isCount(policy.window_ns) || policy.window_ns < 1e9 || policy.window_ns > 3600e9 || !isCount(policy.min_targets) || policy.min_targets < 2 || policy.min_targets > 128) return false;
  if (!Array.isArray(value.members) || value.members.length > 128) return false;
  const identities = new Set<string>(), targets = new Set<string>();
  for (const member of value.members) {
    if (!isRecord(member) || (member.type !== "tcp" && member.type !== "dns") || typeof member.target !== "string" || !member.target || member.target.length > 512) return false;
    const key = `${member.type}:${member.target}`;
    if (identities.has(key)) return false;
    identities.add(key); targets.add(member.target);
  }
  return targets.size >= policy.min_targets;
}

export function parseCorrelations(value: unknown): CorrelationReport {
  if (!isRecord(value) || !["5m", "15m", "1h", "6h", "24h", "all"].includes(String(value.range)) || !isTimestamp(value.as_of) || !isCount(value.count) || !Array.isArray(value.correlations) || value.correlations.length > 200 || value.count !== value.correlations.length || !value.correlations.every(validEvent) || typeof value.has_more !== "boolean" || typeof value.next_cursor !== "string" || value.next_cursor.length > 256 || value.has_more !== (value.next_cursor.length > 0) || new Set(value.correlations.map(e => e.id)).size !== value.count) throw new Error("Invalid correlation response");
  return value as unknown as CorrelationReport;
}
