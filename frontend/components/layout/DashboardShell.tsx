"use client";

import { usePathname, useRouter } from "next/navigation";
import { navigation, navigationGroups } from "@/lib/navigation";
import type { ReactNode } from "react";

import DashboardSidebar from "@/components/dashboard/DashboardSidebar";
import DashboardHeader from "@/components/dashboard/DashboardHeader";
import TimeRangeSelector from "@/components/dashboard/TimeRangeSelector";

import { useMonitoring } from "@/context/MonitoringContext";

interface Props {
  children: ReactNode;
}

export default function DashboardShell({ children }: Props) {
  const pathname = usePathname();
  const router = useRouter();
  const { status, reconnect, historyLoading, historyError, stale } =
    useMonitoring();

  return (
    <main className="flex min-h-screen bg-[#0B1016] text-white">
      <DashboardSidebar />

      <div className="min-w-0 flex-1">
        <DashboardHeader status={status} onReconnect={reconnect} />

        <div className="border-b border-white/[0.07] px-6 py-4">
          <TimeRangeSelector />
        </div>

        <nav
          aria-label="Mobile navigation"
          className="border-b border-white/[0.07] px-4 py-3 lg:hidden"
        >
          <label className="flex items-center gap-3 text-xs text-slate-400">
            View
            <select
              aria-label="Dashboard page"
              className="min-w-0 flex-1 rounded-lg border border-white/10 bg-[#111820] px-3 py-2 text-sm text-slate-200"
              value={pathname}
              onChange={(event) => router.push(event.target.value)}
            >
              {navigationGroups.map((group) => (
                <optgroup key={group} label={group}>
                  {navigation
                    .filter((item) => item.group === group)
                    .map((item) => (
                      <option key={item.href} value={item.href}>
                        {item.label}
                      </option>
                    ))}
                </optgroup>
              ))}
            </select>
          </label>
        </nav>
        {(historyLoading ||
          historyError ||
          stale ||
          status !== "connected") && (
          <div
            role="status"
            className="space-y-1 px-6 py-3 text-sm text-amber-300"
          >
            {historyLoading && <p>Loading historical measurements...</p>}
            {historyError && <p>{historyError}</p>}
            {stale && (
              <p>
                Measurements are stale. No new snapshot has arrived in over 15
                seconds.
              </p>
            )}
            {status !== "connected" && (
              <p>
                Live monitoring is {status}. Showing available measurements.
              </p>
            )}
          </div>
        )}
        {children}
      </div>
    </main>
  );
}
