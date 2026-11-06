
"use client";

import Link from "next/link";
import type { ReactNode } from "react";

import DashboardSidebar from "@/components/dashboard/DashboardSidebar";
import DashboardHeader from "@/components/dashboard/DashboardHeader";
import TimeRangeSelector from "@/components/dashboard/TimeRangeSelector";

import { useMonitoring } from "@/context/MonitoringContext";

interface Props {
  children: ReactNode;
}

export default function DashboardShell({ children }: Props) {
  const { status, reconnect, historyLoading, historyError, stale } = useMonitoring();

  return (
    <main className="flex min-h-screen bg-[#0B1016] text-white">
      <DashboardSidebar />

      <div className="min-w-0 flex-1">
        <DashboardHeader
          status={status}
          onReconnect={reconnect}
        />

        <div className="border-b border-white/[0.07] px-6 py-4">
          <TimeRangeSelector />
        </div>

        <nav aria-label="Mobile navigation" className="flex flex-wrap gap-4 border-b border-white/[0.07] px-5 py-3 text-sm lg:hidden">
          <Link href="/">Overview</Link>
          <Link href="/tcp">TCP</Link>
          <Link href="/dns">DNS</Link>
          <Link href="/health">Health</Link>
          <Link href="/learn">Metrics guide</Link>
        </nav>
        {(historyLoading || historyError || stale || status !== "connected") && (
          <div role="status" className="space-y-1 px-6 py-3 text-sm text-amber-300">
            {historyLoading && <p>Loading historical measurements...</p>}
            {historyError && <p>{historyError}</p>}
            {stale && <p>Measurements are stale. No new snapshot has arrived in over 15 seconds.</p>}
            {status !== "connected" && <p>Live monitoring is {status}. Showing available measurements.</p>}
          </div>
        )}
        {children}
      </div>
    </main>
  );
}
