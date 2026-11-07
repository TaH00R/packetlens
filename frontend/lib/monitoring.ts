import type { MonitoringSnapshot, ProbeResult } from "../types/metrics";

export type TimeRange = "5m" | "15m" | "1h" | "6h" | "24h" | "all";

// Bound the browser cache independently of the durable backend history.
export const HISTORY_CAPACITY = 1000;

const RANGE_MS = { "5m": 300_000, "15m": 900_000, "1h": 3_600_000, "6h": 21_600_000, "24h": 86_400_000 };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isTimestamp(value: unknown): value is string {
  return typeof value === "string" && Number.isFinite(Date.parse(value));
}

function isProbe(value: unknown, type: ProbeResult["type"]): value is ProbeResult {
  return isRecord(value) && value.type === type &&
    typeof value.target === "string" && value.target.length > 0 &&
    typeof value.latency_ms === "number" && Number.isFinite(value.latency_ms) && value.latency_ms >= 0 &&
    typeof value.success === "boolean" && isTimestamp(value.timestamp) &&
    (value.error === undefined || typeof value.error === "string");
}

export function isSnapshot(value: unknown): value is MonitoringSnapshot {
  return isRecord(value) && isTimestamp(value.timestamp) &&
    (value.id === undefined || (typeof value.id === "string" && /^[a-f0-9]{32}$/.test(value.id))) &&
    (value.dns_comparisons === undefined || (Array.isArray(value.dns_comparisons) && value.dns_comparisons.length<=96 && value.dns_comparisons.every(p => isProbe(p,"dns") && isRecord(p) && typeof p.resolver === "string" && p.resolver.length>0 && p.resolver.length<=128 && (p.success ? p.outcome === "success" : ["timeout","network_error","dns_error","no_answer","invalid_response"].includes(String(p.outcome)))))) &&
    Array.isArray(value.tcp) && value.tcp.every((probe) => isProbe(probe, "tcp")) &&
    Array.isArray(value.dns) && value.dns.every((probe) => isProbe(probe, "dns"));
}

export function parseHistory(value: unknown): MonitoringSnapshot[] {
  if (!isRecord(value) || !Array.isArray(value.snapshots) ||
      value.count !== value.snapshots.length || !value.snapshots.every(isSnapshot)) {
    throw new Error("Invalid snapshot history response");
  }
  return value.snapshots;
}

// Include the measurements in the identity: timestamps alone are not unique.
function snapshotKey(snapshot: MonitoringSnapshot): string {
  if (snapshot.id) return `id:${snapshot.id}`;
  const probes = (results: ProbeResult[]) => results.map((result) =>
    JSON.stringify([result.type, result.target, result.timestamp, result.success, result.latency_ms, result.error ?? ""])
  ).sort();
  return JSON.stringify([snapshot.timestamp, probes(snapshot.tcp), probes(snapshot.dns), (snapshot.dns_comparisons ?? []).map(p => JSON.stringify([p.resolver,p.outcome,probes([p])[0]])).sort()]);
}

export function mergeHistory(...histories: MonitoringSnapshot[][]): MonitoringSnapshot[] {
  const unique = new Map<string, MonitoringSnapshot>();
  for (const snapshot of histories.flat()) unique.set(snapshotKey(snapshot), snapshot);
  return [...unique.values()].sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp)).slice(-HISTORY_CAPACITY);
}

export function filterHistory(history: MonitoringSnapshot[], range: TimeRange, now: number): MonitoringSnapshot[] {
  if (range === "all") return history;
  const cutoff = now - RANGE_MS[range];
  return history.filter((snapshot) => Date.parse(snapshot.timestamp) >= cutoff && Date.parse(snapshot.timestamp) <= now);
}
