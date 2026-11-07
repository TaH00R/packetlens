
export interface ProbeResult {
  type: "tcp" | "dns";
  target: string;
  latency_ms: number;
  success: boolean;
  error?: string;
  timestamp: string;
}

export interface DNSComparison extends ProbeResult {
 type: "dns"; resolver: string; outcome: "success" | "timeout" | "network_error" | "dns_error" | "no_answer" | "invalid_response";
}

export interface MonitoringSnapshot {
 dns_comparisons?: DNSComparison[];
  id?: string;
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
