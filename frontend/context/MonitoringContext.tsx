
"use client";

import {
  createContext,
  useContext,
  useMemo,
  type ReactNode,
} from "react";

import { usePacketLensSocket } from "@/hooks/usePacketLensSocket";

import type {
  ConnectionStatus,
  MonitoringSnapshot,
} from "@/types/metrics";

interface MonitoringContextType {
  status: ConnectionStatus;
  latest: MonitoringSnapshot | null;
  history: MonitoringSnapshot[];
  reconnect: () => void;
}

const MonitoringContext =
  createContext<MonitoringContextType | undefined>(undefined);

export function MonitoringProvider({
  children,
}: {
  children: ReactNode;
}) {
  const monitoring = usePacketLensSocket();

  const value = useMemo(
    () => ({
      status: monitoring.status,
      latest: monitoring.latest,
      history: monitoring.history,
      reconnect: monitoring.reconnect,
    }),
    [
      monitoring.status,
      monitoring.latest,
      monitoring.history,
      monitoring.reconnect,
    ]
  );

  return (
    <MonitoringContext.Provider value={value}>
      {children}
    </MonitoringContext.Provider>
  );
}

export function useMonitoring() {
  const context = useContext(MonitoringContext);

  if (!context) {
    throw new Error(
      "useMonitoring must be used inside MonitoringProvider"
    );
  }

  return context;
}
