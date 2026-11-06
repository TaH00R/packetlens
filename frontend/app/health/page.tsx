"use client";

import AnomalyFeed from "@/components/analytics/AnomalyFeed";

import AvailabilityPanel from "@/components/analytics/AvailabilityPanel";
import JitterHealth from "@/components/analytics/JitterHealth";

export default function NetworkHealthPage() {
  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section><h1 className="text-3xl font-semibold">Network Health</h1><p className="mt-2 text-sm text-gray-400">Observed probe availability and per-target latency consistency.</p></section>
      <AvailabilityPanel />
      <JitterHealth />
      <AnomalyFeed />
      <p className="rounded-xl border border-amber-500/15 bg-amber-500/5 p-5 text-sm leading-6 text-gray-300">These TCP and DNS checks describe configured operations. Probe success is not packet-level delivery, and matching failures do not establish a shared root cause.</p>
    </div>
  );
}
