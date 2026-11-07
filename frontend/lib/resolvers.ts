import type { ResolverReport, ResolverSummary } from "../types/resolvers";
import { isRecord, isCount, isTimestamp, isNonnegative } from "./validation";
const nullableNumber = (v: unknown) => v === null || isNonnegative(v);
function summary(v: unknown): v is ResolverSummary {
  if (!isRecord(v) || typeof v.resolver !== "string" || !v.resolver || v.resolver.length > 128 || typeof v.target !== "string" || v.target.length > 512 || ![v.attempts, v.successful, v.failed, v.timeouts].every(isCount)) return false;
  const attempts = Number(v.attempts), successes = Number(v.successful), failures = Number(v.failed);
  if (attempts !== successes + failures || Number(v.timeouts) > failures || ![v.avg_latency_ms, v.min_latency_ms, v.max_latency_ms].every(nullableNumber) || (successes === 0 ? v.avg_latency_ms !== null || v.min_latency_ms !== null || v.max_latency_ms !== null : !isNonnegative(v.avg_latency_ms) || !isNonnegative(v.min_latency_ms) || !isNonnegative(v.max_latency_ms) || v.min_latency_ms > v.avg_latency_ms || v.avg_latency_ms > v.max_latency_ms)) return false;
  if ((v.last_failure_at !== null && !isTimestamp(v.last_failure_at)) || (failures === 0) !== (v.last_failure_at === null) || typeof v.last_error !== "string" || v.last_error.length > 1024 || !Array.isArray(v.trend) || v.trend.length !== 60) return false;
  let total = 0, successful = 0, previous = -Infinity;
  for (const p of v.trend) {
    if (!isRecord(p) || !isTimestamp(p.timestamp) || Date.parse(p.timestamp) < previous || !isCount(p.attempts) || !isCount(p.successful) || p.successful > p.attempts || (p.successful === 0 ? p.avg_latency_ms !== null : !isNonnegative(p.avg_latency_ms)) || (p.attempts === 0 ? p.availability !== null : !isNonnegative(p.availability) || Math.abs(p.availability - 100 * p.successful / p.attempts) > 1e-6)) return false;
    previous = Date.parse(p.timestamp); total += p.attempts; successful += p.successful;
  }
  return total === attempts && successful === successes;
}
export function parseResolvers(value: unknown): ResolverReport {
  if (!isRecord(value) || !["5m", "15m", "1h", "6h", "24h", "all"].includes(String(value.range)) || ![value.as_of, value.from, value.to].every(isTimestamp) || Date.parse(String(value.from)) > Date.parse(String(value.to)) || value.to !== value.as_of || !Array.isArray(value.configured_resolvers) || value.configured_resolvers.length > 3 || !value.configured_resolvers.every(v => typeof v === "string" && v.length > 0 && v.length <= 128) || new Set(value.configured_resolvers).size !== value.configured_resolvers.length || !Array.isArray(value.groups) || !Array.isArray(value.targets) || value.groups.length > 128 || value.targets.length > 128 || !value.groups.every(summary) || !value.targets.every(summary)) throw new Error("Invalid DNS resolver report");
  if (value.groups.some(g => g.target !== "") || value.targets.some(t => !t.target) || new Set(value.groups.map(g => g.resolver)).size !== value.groups.length || new Set(value.targets.map(t => `${t.resolver}|${t.target}`)).size !== value.targets.length) throw new Error("Invalid DNS resolver identities");
  for (const g of value.groups) { const targets = value.targets.filter(t => t.resolver === g.resolver); if (targets.reduce((n, t) => n + t.attempts, 0) !== g.attempts || targets.reduce((n, t) => n + t.successful, 0) !== g.successful) throw new Error("Inconsistent DNS resolver totals") }
  const groups = value.groups;
  if (value.targets.some(t => !groups.some(g => g.resolver === t.resolver))) throw new Error("Missing DNS resolver group");
  return value as unknown as ResolverReport;
}
