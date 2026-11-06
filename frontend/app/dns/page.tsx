
"use client";

import {
  Activity,
  Globe,
  CheckCircle2,
  Timer,
} from "lucide-react";

import MetricCard from "@/components/dashboard/MetricCard";
import TargetLatencyChart from "@/components/dashboard/TargetLatencyChart";
import ProbeFeed from "@/components/dashboard/ProbeFeed";

import { useMonitoring } from "@/context/MonitoringContext";

export default function DNSAnalyticsPage() {
  const { history } = useMonitoring();

  const results = history.flatMap((snapshot) => snapshot.dns);

  const successful = results.filter((result) => result.success);

  const average =
    successful.length > 0
      ? successful.reduce(
          (sum, result) => sum + result.latency_ms,
          0
        ) / successful.length
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

  const domains = new Set(results.map((result) => result.target));

  const dnsSnapshots = history.map((snapshot) => ({
    ...snapshot,
    tcp: [],
  }));

  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section>
        <div className="mb-3 inline-flex rounded-md border border-blue-500/20 bg-blue-500/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-blue-400">
          Workspace / DNS
        </div>

        <h1 className="text-3xl font-semibold">
          DNS Analytics
        </h1>

        <p className="mt-2 text-sm text-gray-500">
          Monitor domain resolution performance and lookup reliability.
        </p>
      </section>

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="Average resolution"
          value={average === null ? "—" : `${average.toFixed(2)} ms`}
          description="Successful DNS lookups"
          icon={Activity}
          color="text-blue-400"
        />

        <MetricCard
          title="Fastest resolution"
          value={min === null ? "—" : `${min.toFixed(2)} ms`}
          description="Lowest observed lookup time"
          icon={Timer}
          color="text-teal-400"
        />

        <MetricCard
          title="Lookup success rate"
          value={successRate === null ? "—" : `${successRate.toFixed(1)}%`}
          description={`${successful.length} successful / ${results.length} total`}
          icon={CheckCircle2}
          color="text-emerald-400"
        />

        <MetricCard
          title="Monitored domains"
          value={String(domains.size)}
          description="Unique DNS targets observed"
          icon={Globe}
          color="text-purple-400"
        />
      </section>

      <TargetLatencyChart
        results={dnsSnapshots}
        initialType="dns"
        availableTypes={["dns"]}
      />

      <ProbeFeed results={results} />
    </div>
  );
}
