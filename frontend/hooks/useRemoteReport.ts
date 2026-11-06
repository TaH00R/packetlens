"use client";

import { useEffect, useState } from "react";
import { loadReport } from "@/lib/analytics";
import type { RemoteReport } from "@/types/analytics";

const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

export function useRemoteReport<T>(path: string, parse: (value: unknown) => T, intervalMs: number, connectionVersion: number): RemoteReport<T> {
  const [state, setState] = useState<RemoteReport<T> & { path: string }>({ path, data: null, loading: true, error: null });
  useEffect(() => {
    let disposed = false;
    let active = false;
    let controller: AbortController | null = null;
    let timeout: ReturnType<typeof setTimeout> | null = null;
    async function refresh() {
      if (active) return;
      active = true;
      controller = new AbortController();
      timeout = setTimeout(() => controller?.abort(), 35_000);
      setState((previous) => ({ path, data: previous.path === path ? previous.data : null, loading: true, error: null }));
      try {
        const data = await loadReport(`${API_URL}${path}`, parse, controller.signal);
        if (!disposed) setState({ path, data, loading: false, error: null });
      } catch (error) {
        if (!disposed) {
          console.error("[PacketLens] Analytics load failed:", error);
          setState((previous) => ({ path, data: previous.path === path ? previous.data : null, loading: false, error: "Stored analytics could not be refreshed. Showing the last available report, if any." }));
        }
      } finally {
        if (timeout) clearTimeout(timeout);
        active = false;
      }
    }
    void refresh();
    const timer = setInterval(() => void refresh(), intervalMs);
    return () => { disposed = true; clearInterval(timer); if (timeout) clearTimeout(timeout); controller?.abort(); };
  }, [path, parse, intervalMs, connectionVersion]);
  return state.path === path ? state : { data: null, loading: true, error: null };
}
