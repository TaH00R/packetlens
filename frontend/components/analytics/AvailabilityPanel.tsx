"use client";

import { useState } from "react";
import { ShieldCheck } from "lucide-react";
import { useMonitoring } from "@/context/MonitoringContext";
import { formatRate } from "@/lib/analytics";
import type { ProbeType, AvailabilityWindow } from "@/types/analytics";
import MetricCard from "@/components/dashboard/MetricCard";
import TrendChart from "./TrendChart";

const time = (value: string | null) => value ? new Date(value).toLocaleString() : "None observed";

export default function AvailabilityPanel({ type }: { type?: ProbeType }) {
  const { availability, availabilityStale } = useMonitoring();
  const [localType, setLocalType] = useState<ProbeType>("tcp");
  const [window, setWindow] = useState<AvailabilityWindow["window"]>("24h");
  const selectedType = type ?? localType;
  const report = availability.data;
  const group = report?.groups.find((g) => g.type === selectedType);
  const targets = report?.targets.filter((t) => t.type === selectedType) ?? [];
  const selectedWindow = group?.windows.find((w) => w.window === window);
  return (
    <section className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div><h2 className="text-lg font-medium">Observed availability</h2><p className="mt-1 max-w-3xl text-xs leading-5 text-gray-400">Fixed windows independent of the global range. Successes / observed attempts; missing measurements are unknown. This is probe availability, not time-based service uptime.</p>{report && <p className="text-xs text-gray-400">As of {time(report.as_of)}. Streaks and last outcomes cover retained observations in the last 30 days.</p>}</div>
        {!type && <label className="text-sm">Probe type <select className="ml-2 rounded-lg border border-white/10 bg-[#111820] p-2" value={selectedType} onChange={(e) => setLocalType(e.target.value as ProbeType)}><option value="tcp">TCP</option><option value="dns">DNS</option></select></label>}
      </div>
      {availability.loading && <p role="status" className="text-sm text-gray-400">{report ? "Refreshing availability..." : "Loading availability..."}</p>}
      {availabilityStale && <p role="status" className="text-sm text-amber-300">This stored report is stale. Check its observation time before interpreting the results.</p>}
      {availability.error && <p role="alert" className="text-sm text-amber-300">{availability.error}</p>}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{(["1h", "24h", "7d", "30d"] as const).map((name) => {
        const w = group?.windows.find((w) => w.window === name);
        return <MetricCard key={name} title={`${selectedType.toUpperCase()} availability · ${name}`} value={formatRate(w?.availability)} description={`${w?.successful ?? 0} successful / ${w?.attempts ?? 0} observed`} icon={ShieldCheck} color="text-teal-400" />;
      })}</div>
      <label className="block text-sm text-gray-300">Availability trend window <select className="ml-3 rounded-lg border border-white/10 bg-[#111820] p-2" value={window} onChange={(e) => setWindow(e.target.value as AvailabilityWindow["window"])}>{["1h", "24h", "7d", "30d"].map((w) => <option key={w}>{w}</option>)}</select></label>
      {selectedWindow && <p className="text-xs leading-5 text-gray-400">Requested window: {time(selectedWindow.from)} to {time(selectedWindow.to)}. First / last observation: {time(selectedWindow.first_observed)} / {time(selectedWindow.last_observed)}. {report?.history_start && Date.parse(report.history_start) > Date.parse(selectedWindow.from) ? "Earlier history is not retained; this window is partial." : "Unmeasured intervals are not inferred to be available."}</p>}
      <TrendChart title={`${selectedType.toUpperCase()} availability trend`} description="Observed successes as a percentage of attempts in each bucket. No attempts means a gap." points={selectedWindow?.trend ?? []} metric="availability" />
      <div className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#111820]"><h3 className="p-5 font-medium">Target availability</h3><div className="overflow-x-auto"><table className="w-full text-left text-sm">
        <thead className="border-y border-white/[0.07] text-xs text-gray-400"><tr>{["Target", "1h", "24h", "7d", "30d", "Observed streak", "Last success", "Last failure"].map((label) => <th className="whitespace-nowrap p-4 font-medium" key={label}>{label}</th>)}</tr></thead>
        <tbody>{targets.map((t) => <tr className="border-b border-white/5 text-gray-300" key={t.target}><td className="p-4">{t.target}</td>{t.windows.map((w) => <td className="whitespace-nowrap p-4" key={w.window}>{formatRate(w.availability)}<span className="block text-xs text-gray-400">{w.attempts} attempts</span></td>)}<td className="p-4">{t.consecutive_failures > 0 ? `${t.consecutive_failures} failures` : `${t.consecutive_successes} successes`}</td><td className="whitespace-nowrap p-4">{time(t.last_success)}</td><td className="whitespace-nowrap p-4">{time(t.last_failure)}</td></tr>)}</tbody>
      </table></div>{targets.length === 0 && <p className="p-5 text-sm text-gray-400">No {selectedType.toUpperCase()} observations in the last 30 days.</p>}</div>
    </section>
  );
}
