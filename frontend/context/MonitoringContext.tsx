
"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { usePacketLensSocket } from "@/hooks/usePacketLensSocket";

import type {
  ConnectionStatus,
  MonitoringSnapshot,
} from "@/types/metrics";

import { filterHistory, mergeHistory, parseHistory, type TimeRange } from "@/lib/monitoring";
export type { TimeRange } from "@/lib/monitoring";

interface MonitoringContextType {
  status: ConnectionStatus;
  latest: MonitoringSnapshot | null;
  history: MonitoringSnapshot[];
  allHistory: MonitoringSnapshot[];
  historyLoading: boolean;
  historyError: string | null;
  stale: boolean;
  range: TimeRange;
  setRange: (range: TimeRange) => void;
  reconnect: () => void;
}

const MonitoringContext =
  createContext<MonitoringContextType | undefined>(undefined);

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export function MonitoringProvider({
  children,
}: {
  children: ReactNode;
}) {
  const monitoring = usePacketLensSocket();

  const [historicalSnapshots, setHistoricalSnapshots] =
    useState<MonitoringSnapshot[]>([]);

  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState<string | null>(null);

  const [range, setRange] = useState<TimeRange>("15m");
  const [now, setNow] = useState(0);

  useEffect(() => {
    const updateNow = () => setNow(Date.now());

    updateNow();
    const interval = window.setInterval(updateNow, 5_000);

    return () => window.clearInterval(interval);
  }, [historicalSnapshots, monitoring.latest]);

  // Load snapshots already collected by the backend.
  useEffect(() => {
    const controller = new AbortController();
    let disposed = false;
    const timeout = window.setTimeout(() => controller.abort(), 10_000);

    async function fetchSnapshots() {
      try {
        setHistoryLoading(true);
        setHistoryError(null);
        const response = await fetch(`${API_URL}/api/snapshots`, {
          method: "GET",
          cache: "no-store",
          signal: controller.signal,
        });

        if (!response.ok) {
          throw new Error(
            `Failed to fetch snapshots: ${response.status}`
          );
        }

        const snapshots = parseHistory(await response.json());
        if (!disposed) setHistoricalSnapshots(snapshots);
      } catch (error) {
        if (disposed) return;
        setHistoryError("Historical measurements could not be loaded. Live monitoring remains available.");
        console.error(
          "[PacketLens] Failed to load historical snapshots:",
          error
        );
      } finally {
        window.clearTimeout(timeout);
        if (!disposed) setHistoryLoading(false);
      }
    }

    fetchSnapshots();

    return () => {
      disposed = true;
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [monitoring.connectionVersion]);

  const allHistory = useMemo(
    () => mergeHistory(historicalSnapshots, monitoring.history),
    [historicalSnapshots, monitoring.history]
  );

  const history = useMemo(
    () => filterHistory(allHistory, range, now),
    [allHistory, now, range]
  );

  const latest = useMemo(() => {
    if (allHistory.length === 0) {
      return null;
    }

    return allHistory[allHistory.length - 1];
  }, [allHistory]);

  const stale = latest !== null && now - Date.parse(latest.timestamp) > 15_000;

  const value = useMemo(
    () => ({
      status: monitoring.status,
      latest,
      history,
      allHistory,
      historyLoading,
      historyError,
      stale,
      range,
      setRange,
      reconnect: monitoring.reconnect,
    }),
    [
      monitoring.status,
      monitoring.reconnect,
      latest,
      history,
      allHistory,
      historyLoading,
      historyError,
      stale,
      range,
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
