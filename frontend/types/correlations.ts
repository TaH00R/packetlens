import type { TimeRange } from "@/lib/monitoring";

export interface CorrelationEvent {
  id: string;
  cycle_timestamp: string;
  detected_at: string;
  last_correlated_at: string;
  ended_at: string | null;
  end_cycle: string | null;
  members: { type: "tcp" | "dns"; target: string }[];
  policy: { window_ns: number; min_targets: number };
}
export interface CorrelationReport {
  range: TimeRange;
  as_of: string;
  count: number;
  correlations: CorrelationEvent[];
  has_more: boolean;
  next_cursor: string;
}
