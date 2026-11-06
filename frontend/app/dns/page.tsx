"use client";

import AnomalyFeed from "@/components/analytics/AnomalyFeed";

import { useMonitoring } from "@/context/MonitoringContext";
import LatencyAnalytics from "@/components/analytics/LatencyAnalytics";
import AvailabilityPanel from "@/components/analytics/AvailabilityPanel";
import ProbeFeed from "@/components/dashboard/ProbeFeed";

export default function Page() {
  const { history } = useMonitoring();
  const recent = history.flatMap((snapshot) => snapshot.dns);
  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section><h1 className="text-3xl font-semibold">DNS Analytics</h1><p className="mt-2 text-sm text-gray-400">Resolution timing, latency variation, and observed lookup availability.</p></section>
      <LatencyAnalytics type="dns" />
      <AnomalyFeed />
      <AvailabilityPanel type="dns" />
      <section><p className="mb-3 text-xs text-gray-400">Recent activity uses the latest 1,000 cached cycles within the selection.</p><ProbeFeed results={recent} /></section>
    </div>
  );
}
