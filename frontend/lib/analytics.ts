import type { AnalyticsReport, AvailabilityReport } from "../types/analytics";

type RecordValue = Record<string, unknown>;
function record(value: unknown): value is RecordValue { return typeof value === "object" && value !== null }
function nonnegative(value: unknown): value is number { return typeof value === "number" && Number.isFinite(value) && value >= 0 }
function count(value: unknown): value is number { return nonnegative(value) && Number.isSafeInteger(value) }
function nullableNumber(value: unknown) { return value === null || nonnegative(value) }
function rate(value: unknown) { return value === null || (nonnegative(value) && value <= 100) }
function timestamp(value: unknown): value is string { return typeof value === "string" && Number.isFinite(Date.parse(value)) }
function nullableTime(value: unknown) { return value === null || timestamp(value) }
function identity(v: RecordValue, target: boolean) {
  return (v.type === "tcp" || v.type === "dns") && (target ? typeof v.target === "string" && v.target.length > 0 : v.target === undefined);
}
function counts(v: RecordValue) { return count(v.attempts) && count(v.successful) && v.successful <= v.attempts && rate(v.availability) && (v.attempts === 0 ? v.availability === null : v.availability !== null) }
function periodBounds(v: RecordValue) { return timestamp(v.from) && timestamp(v.to) && Date.parse(v.from) <= Date.parse(v.to) }
function summary(value: unknown, target: boolean): boolean {
  if (!record(value) || !identity(value, target) || !counts(value) || !count(value.failed) || value.failed !== (value.attempts as number) - (value.successful as number)) return false;
  const latencyKeys = ["avg_latency_ms", "min_latency_ms", "max_latency_ms", "p50_ms", "p90_ms", "p95_ms", "p99_ms"];
  if (!latencyKeys.every((key) => nullableNumber(value[key]) && (value.successful === 0 ? value[key] === null : value[key] !== null))) return false;
  if (!nullableNumber(value.jitter_ms) || !nullableNumber(value.current_jitter_ms) || !nullableTime(value.first_observed) || !nullableTime(value.last_observed)) return false;
  if (!Array.isArray(value.histogram) || value.histogram.length !== 8 || !value.histogram.every((b) => record(b) && nonnegative(b.lower_ms) && nullableNumber(b.upper_ms) && count(b.count) && (b.upper_ms === null || (b.upper_ms as number) > b.lower_ms))) return false;
  return value.histogram.reduce((sum: number, b: RecordValue) => sum + (b.count as number), 0) === value.successful;
}
function point(value: unknown): boolean {
  return record(value) && timestamp(value.timestamp) && counts(value) && nullableNumber(value.avg_latency_ms) && nullableNumber(value.jitter_ms) && (value.successful !== 0 || value.avg_latency_ms === null);
}
function series(value: unknown, target: boolean): boolean { return record(value) && identity(value, target) && Array.isArray(value.points) && value.points.length === 60 && value.points.every(point) }
function twoGroups(values: unknown, validate: (v: unknown) => boolean): boolean {
  return Array.isArray(values) && values.length === 2 && values.every(validate) && values.some((v) => v.type === "tcp") && values.some((v) => v.type === "dns");
}
function period(value: unknown, trends: boolean): boolean {
  return record(value) && periodBounds(value) && twoGroups(value.groups, (v) => summary(v, false)) &&
    Array.isArray(value.targets) && value.targets.length <= 128 && value.targets.every((v) => summary(v, true)) &&
    Array.isArray(value.trends) && (trends ? twoGroups(value.trends, (v) => series(v, false)) : value.trends.length === 0) &&
    Array.isArray(value.target_trends) && value.target_trends.length <= 128 && value.target_trends.every((v) => series(v, true));
}
export function parseAnalytics(value: unknown): AnalyticsReport {
  if (!record(value) || !["5m", "15m", "1h", "6h", "24h", "all"].includes(String(value.range)) || !timestamp(value.as_of) || !nullableTime(value.history_start) ||
      !period(value.current, true) || (value.range === "all" ? value.previous !== null : !period(value.previous, false))) throw new Error("Invalid analytics response");
  return value as unknown as AnalyticsReport;
}
function availabilityTarget(value: unknown, target: boolean): boolean {
  if (!record(value) || !identity(value, target) || !count(value.consecutive_successes) || !count(value.consecutive_failures) || (value.consecutive_successes > 0 && value.consecutive_failures > 0) || !nullableTime(value.last_success) || !nullableTime(value.last_failure) || !Array.isArray(value.windows) || value.windows.length !== 4) return false;
  return ["1h", "24h", "7d", "30d"].every((name, i) => {
    const w = value.windows instanceof Array ? value.windows[i] : null;
    return record(w) && w.window === name && periodBounds(w) && counts(w) && nullableTime(w.first_observed) && nullableTime(w.last_observed) && Array.isArray(w.trend) && w.trend.length === (target ? 0 : 60) && w.trend.every(point);
  });
}
export function parseAvailability(value: unknown): AvailabilityReport {
  if (!record(value) || !timestamp(value.as_of) || !nullableTime(value.history_start) || !twoGroups(value.groups, (v) => availabilityTarget(v, false)) ||
      !Array.isArray(value.targets) || value.targets.length > 128 || !value.targets.every((v) => availabilityTarget(v, true))) throw new Error("Invalid availability response");
  return value as unknown as AvailabilityReport;
}

export async function loadReport<T>(url: string, parse: (value: unknown) => T, signal: AbortSignal, fetcher: typeof fetch = fetch): Promise<T> {
  const response = await fetcher(url, { signal, cache: "no-store" });
  if (!response.ok) throw new Error(`Analytics request failed (${response.status})`);
  return parse(await response.json());
}

export function formatMs(value: number | null | undefined): string { return value == null ? "No data" : `${value.toFixed(2)} ms` }
export function formatRate(value: number | null | undefined): string { return value == null ? "No observations" : `${value.toFixed(2)}%` }
export function percentChange(current: number | null | undefined, previous: number | null | undefined): string {
  if (current == null || previous == null || previous === 0) return "No comparable baseline";
  const change = 100 * (current-previous)/previous;
  return `${change >= 0 ? "+" : ""}${change.toFixed(1)}%`;
}
