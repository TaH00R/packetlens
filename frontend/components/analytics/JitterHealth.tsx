"use client";

import { useState } from "react";
import { useMonitoring } from "@/context/MonitoringContext";
import { formatMs } from "@/lib/analytics";
import TrendChart from "./TrendChart";

export default function JitterHealth() {
  const { analytics, analyticsStale } = useMonitoring();
  const [selected, setSelected] = useState("");
  const targets = analytics.data?.current.targets ?? [];
  const series = analytics.data?.current.target_trends ?? [];
  const active = series.find((s) => `${s.type}:${s.target}` === selected) ?? series[0];
  return (
    <section className="space-y-5">
      <h2 className="text-lg font-medium">Jitter in the global time range</h2>
      <p className="text-xs leading-5 text-gray-400">Mean absolute successive-success latency differences, independently per target. Latest pair is the most recent such difference in the selection. Failures are skipped. This is not RTP interarrival jitter.</p>
      {analytics.loading && <p role="status" className="text-sm text-gray-400">Loading jitter analytics...</p>}
      {analyticsStale && <p role="status" className="text-sm text-amber-300">This stored report is stale. Check its observation time before interpreting the results.</p>}
      {analytics.error && <p role="alert" className="text-sm text-amber-300">{analytics.error}</p>}
      <div className="overflow-x-auto rounded-xl border border-white/[0.07] bg-[#111820]"><table className="w-full text-left text-sm">
        <thead className="text-xs text-gray-400"><tr>{["Type", "Target", "Successful probes", "Mean jitter", "Latest pair"].map((label) => <th className="p-4 font-medium" key={label}>{label}</th>)}</tr></thead>
        <tbody>{targets.map((t) => <tr key={`${t.type}:${t.target}`} className="border-t border-white/5 text-gray-300"><td className="p-4 uppercase">{t.type}</td><td className="p-4">{t.target}</td><td className="p-4">{t.successful}</td><td className="whitespace-nowrap p-4">{formatMs(t.jitter_ms)}</td><td className="whitespace-nowrap p-4">{formatMs(t.current_jitter_ms)}</td></tr>)}</tbody>
      </table>{targets.length === 0 && <p className="p-5 text-sm text-gray-400">No targets observed in this selection.</p>}</div>
      <label className="flex min-w-0 flex-wrap items-center gap-3 text-sm text-gray-300">Jitter target <select className="min-w-0 max-w-full rounded-lg border border-white/10 bg-[#111820] p-2" value={active ? `${active.type}:${active.target}` : ""} onChange={(e) => setSelected(e.target.value)} disabled={series.length === 0}>
        {series.length === 0 && <option value="">No targets</option>}{series.map((s) => <option key={`${s.type}:${s.target}`} value={`${s.type}:${s.target}`}>{s.type.toUpperCase()} · {s.target}</option>)}
      </select></label>
      <TrendChart title="Per-target jitter trend" description="Pairs are assigned to the time bucket containing their newer observation; missing pairs leave gaps." points={active?.points ?? []} metric="jitter_ms" />
    </section>
  );
}
