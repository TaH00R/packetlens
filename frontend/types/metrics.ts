
export interface ProbeResult {
  type: "tcp" | "dns";
  target: string;
  latency_ms: number;
  success: boolean;
  error?: string;
  timestamp: string;
}

export interface MonitoringSnapshot {
  timestamp: string;
  tcp: ProbeResult[];
  dns: ProbeResult[];
}

export type ConnectionStatus =
  | "connecting"
  | "connected"
  | "reconnecting"
  | "disconnected";

export interface LatencyPoint {
  timestamp: number;
  tcp: number | null;
  dns: number | null;
}
