import type { TimeRange } from "@/lib/monitoring";
export interface ResolverPoint { timestamp: string; attempts: number; successful: number; avg_latency_ms: number | null; availability: number | null }
export interface ResolverSummary {
  resolver: string; target: string; attempts: number; successful: number; failed: number; timeouts: number;
  avg_latency_ms: number | null; min_latency_ms: number | null; max_latency_ms: number | null;
  last_failure_at: string | null; last_error: string; trend: ResolverPoint[];
}
export interface ResolverReport { range: TimeRange; as_of: string; from: string; to: string; configured_resolvers: string[]; groups: ResolverSummary[]; targets: ResolverSummary[] }
