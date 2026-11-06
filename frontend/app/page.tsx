
"use client";

import {
  Activity,
  Globe,
  Network,
  ShieldCheck,
} from "lucide-react";

import MetricCard from "@/components/dashboard/MetricCard";
import LatencyChart from "@/components/dashboard/LatencyChart";
import ProbeFeed from "@/components/dashboard/ProbeFeed";
import NetworkHealth from "@/components/dashboard/NetworkHealth";
import TargetLatencyChart from "@/components/dashboard/TargetLatencyChart";

import { useMonitoring } from "@/context/MonitoringContext";
import type { ProbeResult } from "@/types/metrics";

function average(results: ProbeResult[]) {
  const successful = results.filter((result) => result.success);

  if (successful.length === 0) return "—";

  const total = successful.reduce(
    (sum, result) => sum + result.latency_ms,
    0
  );

  return `${(total / successful.length).toFixed(2)} ms`;
}

export default function Home() {
  const { history } = useMonitoring();

  const probeHistory = history.flatMap((snapshot) => [
    ...snapshot.tcp,
    ...snapshot.dns,
  ]);

  const successful = probeHistory.filter((result) => result.success);

  const tcp = successful.filter((result) => result.type === "tcp");
  const dns = successful.filter((result) => result.type === "dns");

  const successRate = probeHistory.length
    ? `${((successful.length / probeHistory.length) * 100).toFixed(1)}%`
    : "—";

  const activeTargets = new Set(
    probeHistory.map((result) => `${result.type}:${result.target}`)
  ).size;

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

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="TCP latency"
          value={average(tcp)}
          description="Average connection establishment time"
          icon={Network}
          color="text-teal-400"
        />

        <MetricCard
          title="DNS resolution"
          value={average(dns)}
          description="Average domain lookup duration"
          icon={Globe}
          color="text-blue-400"
        />

        <MetricCard
          title="Success rate"
          value={successRate}
          description={`${successful.length} successful / ${probeHistory.length} total`}
          icon={ShieldCheck}
          color="text-emerald-400"
        />

        <MetricCard
          title="Active targets"
          value={String(activeTargets)}
          description="Unique targets observed this session"
          icon={Activity}
          color="text-purple-400"
        />
      </section>

      <LatencyChart results={history} />

      <TargetLatencyChart results={history} />

      <section className="grid grid-cols-1 gap-5 xl:grid-cols-[1.5fr_1fr]">
        <NetworkHealth results={probeHistory} />
        <ProbeFeed results={probeHistory} />
      </section>

      <footer className="flex justify-between border-t border-white/[0.07] pt-5 text-xs text-gray-600">
        <p>PacketLens · Network Observability</p>
        <p>Built with Go and Next.js</p>
      </footer>
    </div>
  );
}
