import type { MonitoringTarget, TargetInput } from "../types/targets";
import { isRecord, isCount } from "./validation";

export function parseTarget(value: unknown): MonitoringTarget {
  if (!isRecord(value) || typeof value.id !== "string" || !/^[a-f0-9]{32}$/.test(value.id) || (value.type !== "tcp" && value.type !== "dns") || typeof value.address !== "string" || !value.address || value.address.length > 512 || typeof value.enabled !== "boolean" || !isCount(value.interval_ms) || value.interval_ms < 5000 || value.interval_ms > 3600000 || !isCount(value.timeout_ms) || value.timeout_ms < 100 || value.timeout_ms > 30000 || value.timeout_ms > value.interval_ms) throw new Error("Invalid target response");
  return value as unknown as MonitoringTarget;
}
export function parseTargets(value: unknown): MonitoringTarget[] {
  if (!isRecord(value) || !isCount(value.count) || !Array.isArray(value.targets) || value.count !== value.targets.length || value.count > 32) throw new Error("Invalid target list");
  const targets = value.targets.map(parseTarget);
  if (new Set(targets.map(t => t.id)).size !== targets.length || new Set(targets.map(t => `${t.type}:${t.address}`)).size !== targets.length) throw new Error("Duplicate target response");
  return targets;
}
const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
export async function targetRequest(token: string, method: "GET" | "POST" | "PUT" | "DELETE", id: string | null, signal: AbortSignal, body?: TargetInput): Promise<unknown> {
  const response = await fetch(`${API_URL}/api/targets${id ? `/${encodeURIComponent(id)}` : ""}`, { method, signal, cache: "no-store", headers: { Authorization: `Bearer ${token}`, ...(body ? { "Content-Type": "application/json" } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const value: unknown = await response.json();
  if (!response.ok) {
    const message = isRecord(value) && typeof value.error === "string" ? value.error : `Target request failed (${response.status})`;
    throw new Error(message);
  }
  return value;
}
