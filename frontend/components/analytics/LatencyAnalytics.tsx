"use client";

import { useState } from "react";
import { Activity, Gauge } from "lucide-react";
import { useMonitoring } from "@/context/MonitoringContext";
import { formatMs, formatRate, percentChange } from "@/lib/analytics";
import type { ProbeType } from "@/types/analytics";
import MetricCard from "@/components/dashboard/MetricCard";
import DistributionChart from "./DistributionChart";
import TrendChart from "./TrendChart";

export default function LatencyAnalytics({ type }: { type: ProbeType }) {
  const { analytics, analyticsStale } = useMonitoring();
  const [target, setTarget] = useState("");
  const report = analytics.data;
  const targets = report?.current.targets.filter((t) => t.type === type) ?? [];
  const selected = targets.some((t) => t.target === target) ? target : "";
  const summary = selected ? targets.find((t) => t.target === selected) : report?.current.groups.find((g) => g.type === type);
  const previous = selected ? report?.previous?.targets.find((t) => t.type === type && t.target === selected) : report?.previous?.groups.find((g) => g.type === type);
  const points = (selected ? report?.current.target_trends.find((s) => s.type === type && s.target === selected) : report?.current.trends.find((s) => s.type === type))?.points ?? [];
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="text-lg font-medium">Stored {type.toUpperCase()} performance</h2>
          <p className="mt-1 text-xs leading-5 text-gray-400">All retained measurements in the selected range. Percentiles use successful probes and exact nearest ranks.</p>
          {report && <p className="text-xs text-gray-400">As of {new Date(report.as_of).toLocaleString()} · {summary?.attempts ?? 0} observed attempts</p>}
        </div>
        <label className="flex min-w-0 max-w-full flex-wrap items-center gap-2 text-xs text-gray-300">Target
          <select aria-label={`${type.toUpperCase()} analytics target`} className="min-w-0 max-w-full rounded-lg border border-white/10 bg-[#111820] p-2 text-sm" value={selected} onChange={(e) => setTarget(e.target.value)} disabled={!report || targets.length === 0}>
            <option value="">All {type.toUpperCase()} targets</option>
            {targets.map((t) => <option key={t.target} value={t.target}>{t.target}</option>)}
          </select>
        </label>
      </div>
      {analytics.loading && <p role="status" className="text-sm text-gray-400">{report ? "Refreshing stored analytics..." : "Loading stored analytics..."}</p>}
      {analyticsStale && <p role="status" className="text-sm text-amber-300">This stored report is stale. Check its observation time before interpreting the results.</p>}
      {analytics.error && <p role="alert" className="rounded-lg border border-amber-500/20 p-3 text-sm text-amber-300">{analytics.error}</p>}
      {report?.history_start && Date.parse(report.history_start) > Date.parse(report.current.from) && <p className="text-xs text-amber-300">Retained history begins {new Date(report.history_start).toLocaleString()}. Earlier parts of this selection have no retained data.</p>}
      {!report && !analytics.loading ? <p className="text-sm text-gray-400">Stored analytics are unavailable. Recent live activity remains below.</p> : <>
        <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {([50, 90, 95, 99] as const).map((p) => <MetricCard key={p} title={`P${p} latency`} value={formatMs(summary?.[`p${p}_ms`])} description="Nearest-rank successful latency" icon={Gauge} color="text-teal-400" />)}
          {[["Average latency", summary?.avg_latency_ms], ["Minimum latency", summary?.min_latency_ms], ["Maximum latency", summary?.max_latency_ms], ["Mean jitter", summary?.jitter_ms]].map(([title, value]) => <MetricCard key={String(title)} title={String(title)} value={formatMs(typeof value === "number" ? value : null)} description={title === "Mean jitter" ? "Mean per-target successful latency variation" : "Successful measurements in selection"} icon={Activity} color="text-blue-400" />)}
        </section>
        <section className="rounded-xl border border-white/[0.07] bg-[#111820] p-5">
          <h3 className="font-medium">Previous-period comparison</h3>
          {report?.previous ? <p className="mt-2 text-sm leading-6 text-gray-300">Average: {formatMs(summary?.avg_latency_ms)} versus {formatMs(previous?.avg_latency_ms)} ({percentChange(summary?.avg_latency_ms, previous?.avg_latency_ms)}). P95: {formatMs(summary?.p95_ms)} versus {formatMs(previous?.p95_ms)}. Jitter: {formatMs(summary?.jitter_ms)} versus {formatMs(previous?.jitter_ms)}. Previous period: {new Date(report.previous.from).toLocaleString()} to {new Date(report.previous.to).toLocaleString()} ({previous?.attempts ?? 0} attempts).</p> : <p className="mt-2 text-sm text-gray-400">Select a finite time range to compare it with the preceding equal-length period.</p>}
        </section>
        <div className="grid gap-5 xl:grid-cols-2">
          <TrendChart title="Latency trend" description="Average successful latency per time bucket. Empty and all-failed buckets leave gaps." points={points} metric="avg_latency_ms" />
          <DistributionChart bins={summary?.histogram ?? []} />
        </div>
        <TrendChart title="Jitter trend" description="Mean absolute successive-success differences per bucket, calculated independently per target. Pairs can cross bucket boundaries and failure gaps." points={points} metric="jitter_ms" />
        <section className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#111820]">
          <h3 className="p-5 font-medium">Target comparison</h3>
          <div className="overflow-x-auto"><table className="w-full text-left text-sm">
            <thead className="border-y border-white/[0.07] text-xs text-gray-400"><tr>{["Target", "Attempts", "Success", "Average", "P95", "Mean jitter", "Latest pair"].map((label) => <th className="whitespace-nowrap p-4 font-medium" key={label}>{label}</th>)}</tr></thead>
            <tbody>{targets.map((t) => <tr className="border-b border-white/5 text-gray-300" key={t.target}><td className="p-4">{t.target}</td><td className="p-4">{t.attempts}</td><td className="p-4">{formatRate(t.availability)}</td><td className="whitespace-nowrap p-4">{formatMs(t.avg_latency_ms)}</td><td className="whitespace-nowrap p-4">{formatMs(t.p95_ms)}</td><td className="whitespace-nowrap p-4">{formatMs(t.jitter_ms)}</td><td className="whitespace-nowrap p-4">{formatMs(t.current_jitter_ms)}</td></tr>)}</tbody>
          </table></div>
          {targets.length === 0 && <p className="p-5 text-sm text-gray-400">No targets observed in the selected range.</p>}
        </section>
      </>}
    </div>
  );
}
