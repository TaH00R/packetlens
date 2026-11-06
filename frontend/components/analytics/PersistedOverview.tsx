"use client";

import { Activity, Globe, Network, ShieldCheck } from "lucide-react";
import { useMonitoring } from "@/context/MonitoringContext";
import { formatMs, formatRate } from "@/lib/analytics";
import MetricCard from "@/components/dashboard/MetricCard";
import TrendChart from "./TrendChart";

export default function PersistedOverview() {
  const { analytics, analyticsStale } = useMonitoring();
  const report = analytics.data;
  const tcp = report?.current.groups.find((g) => g.type === "tcp");
  const dns = report?.current.groups.find((g) => g.type === "dns");
  const attempts = (tcp?.attempts ?? 0) + (dns?.attempts ?? 0);
  const successful = (tcp?.successful ?? 0) + (dns?.successful ?? 0);
  return (
    <section className="space-y-5">
      <p className="text-xs text-gray-400">Stored summaries cover every retained measurement in the selected range.{report ? ` As of ${new Date(report.as_of).toLocaleString()}.` : ""}</p>
      {analytics.loading && <p role="status" className="text-sm text-gray-400">Refreshing stored summaries...</p>}
      {analyticsStale && <p role="status" className="text-sm text-amber-300">This stored report is stale. Check its observation time before interpreting the results.</p>}
      {analytics.error && <p role="alert" className="text-sm text-amber-300">{analytics.error}</p>}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard title="TCP latency" value={formatMs(tcp?.avg_latency_ms)} description="Average successful connection time" icon={Network} color="text-teal-400" />
        <MetricCard title="DNS resolution" value={formatMs(dns?.avg_latency_ms)} description="Average successful lookup time" icon={Globe} color="text-blue-400" />
        <MetricCard title="Probe success rate" value={formatRate(attempts > 0 ? 100*successful/attempts : null)} description={`${successful} successful / ${attempts} observed attempts`} icon={ShieldCheck} color="text-emerald-400" />
        <MetricCard title="Observed targets" value={report ? String(report.current.targets.length) : "No data"} description="Distinct probe types and targets in selection" icon={Activity} color="text-purple-400" />
      </div>
      <div className="grid gap-5 xl:grid-cols-2">{(["tcp", "dns"] as const).map((type) => <TrendChart key={type} title={`${type.toUpperCase()} latency trend`} description="Average successful latency in each time bucket across the full stored selection." points={report?.current.trends.find((s) => s.type === type)?.points ?? []} metric="avg_latency_ms" />)}</div>
    </section>
  );
}
