
"use client";

import type { ReactNode } from "react";

import DashboardSidebar from "@/components/dashboard/DashboardSidebar";
import DashboardHeader from "@/components/dashboard/DashboardHeader";

import { useMonitoring } from "@/context/MonitoringContext";

interface Props {
  children: ReactNode;
}

export default function DashboardShell({ children }: Props) {
  const { status, reconnect } = useMonitoring();

  return (
    <main className="flex min-h-screen bg-[#0B1016] text-white">
      <DashboardSidebar status={status} />

      <div className="min-w-0 flex-1">
        <DashboardHeader
          status={status}
          onReconnect={reconnect}
        />

        {children}
      </div>
    </main>
  );
}
