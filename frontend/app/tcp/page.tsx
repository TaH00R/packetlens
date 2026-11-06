"use client";

import AnomalyFeed from "@/components/analytics/AnomalyFeed";

import { useMonitoring } from "@/context/MonitoringContext";
import LatencyAnalytics from "@/components/analytics/LatencyAnalytics";
import AvailabilityPanel from "@/components/analytics/AvailabilityPanel";
import ProbeFeed from "@/components/dashboard/ProbeFeed";

export default function Page() {
  const { history } = useMonitoring();
  const recent = history.flatMap((snapshot) => snapshot.tcp);
  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section><h1 className="text-3xl font-semibold">TCP Analytics</h1><p className="mt-2 text-sm text-gray-400">Connection establishment time, latency variation, and observed availability.</p></section>
      <LatencyAnalytics type="tcp" />
      <AnomalyFeed />
      <AvailabilityPanel type="tcp" />
      <section><p className="mb-3 text-xs text-gray-400">Recent activity uses the latest 1,000 cached cycles within the selection.</p><ProbeFeed results={recent} /></section>
    </div>
  );
}
