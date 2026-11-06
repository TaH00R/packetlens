
"use client";

import { Network, Activity, CheckCircle2, XCircle } from "lucide-react";

import MetricCard from "@/components/dashboard/MetricCard";
import TargetLatencyChart from "@/components/dashboard/TargetLatencyChart";
import ProbeFeed from "@/components/dashboard/ProbeFeed";

import { useMonitoring } from "@/context/MonitoringContext";

export default function TCPAnalyticsPage() {
  const { history } = useMonitoring();

  const results = history.flatMap((snapshot) => snapshot.tcp);

  const successful = results.filter((result) => result.success);

  const average =
    successful.length > 0
      ? successful.reduce((sum, result) => sum + result.latency_ms, 0) /
        successful.length
      : null;

  const min =
    successful.length > 0
      ? Math.min(...successful.map((result) => result.latency_ms))
      : null;

  const max =
    successful.length > 0
      ? Math.max(...successful.map((result) => result.latency_ms))
      : null;

  const successRate =
    results.length > 0
      ? (successful.length / results.length) * 100
      : null;

  const tcpSnapshots = history.map((snapshot) => ({
    ...snapshot,
    dns: [],
  }));

  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section>
        <div className="mb-3 inline-flex rounded-md border border-teal-500/20 bg-teal-500/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-teal-400">
          Workspace / TCP
        </div>

        <h1 className="text-3xl font-semibold">
          TCP Analytics
        </h1>

        <p className="mt-2 text-sm text-gray-500">
          Analyze connection establishment time and destination availability.
        </p>
      </section>

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="Average latency"
          value={average === null ? "—" : `${average.toFixed(2)} ms`}
          description="Successful TCP connections"
          icon={Activity}
          color="text-teal-400"
        />

        <MetricCard
          title="Minimum latency"
          value={min === null ? "—" : `${min.toFixed(2)} ms`}
          description="Best observed connection time"
          icon={Network}
          color="text-blue-400"
        />

        <MetricCard
          title="Success rate"
          value={successRate === null ? "—" : `${successRate.toFixed(1)}%`}
          description={`${successful.length} successful / ${results.length} total`}
          icon={CheckCircle2}
          color="text-emerald-400"
        />

        <MetricCard
          title="Maximum latency"
          value={max === null ? "—" : `${max.toFixed(2)} ms`}
          description="Highest successful measurement"
          icon={XCircle}
          color="text-orange-400"
        />
      </section>

      <TargetLatencyChart results={tcpSnapshots} availableTypes={["tcp"]} />

      <ProbeFeed results={results} />
    </div>
  );
}
