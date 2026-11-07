export interface MonitoringTarget {
  id: string;
  type: "tcp" | "dns";
  address: string;
  enabled: boolean;
  interval_ms: number;
  timeout_ms: number;
}
export type TargetInput = Omit<MonitoringTarget, "id">;
