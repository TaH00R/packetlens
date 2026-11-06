
"use client";

import AnomalyFeed from "@/components/analytics/AnomalyFeed";
import IncidentTimeline from "@/components/analytics/IncidentTimeline";

import PersistedOverview from "@/components/analytics/PersistedOverview";
import ProbeFeed from "@/components/dashboard/ProbeFeed";
import TargetLatencyChart from "@/components/dashboard/TargetLatencyChart";
import { useMonitoring } from "@/context/MonitoringContext";

export default function Home() {
  const { history, anomalies } = useMonitoring();

  const probeHistory = history.flatMap((snapshot) => [
    ...snapshot.tcp,
    ...snapshot.dns,
  ]);

  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section id="overview">
        <div className="mb-3 inline-flex rounded-md border border-teal-500/20 bg-teal-500/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-teal-400">
          Network observability
        </div>

        <h1 className="text-3xl font-semibold tracking-tight">
          Network overview
        </h1>

        <p className="mt-2 text-sm text-gray-500">
          Monitor connectivity, latency and network performance.
        </p>
      </section>

      <PersistedOverview />
      <IncidentTimeline />
      <AnomalyFeed />
      <section>
        <p className="mb-3 text-xs text-gray-400">The following target chart and live activity use up to 1,000 recent cached cycles within the selection.</p>
        <TargetLatencyChart results={history} anomalies={anomalies.data?.anomalies} />
      </section>
      <ProbeFeed results={probeHistory} />

      <footer className="flex justify-between border-t border-white/[0.07] pt-5 text-xs text-gray-600">
        <p>PacketLens · Network Observability</p>
        <p>Built with Go and Next.js</p>
      </footer>
    </div>
  );
}
