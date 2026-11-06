"use client";

import { useState } from "react";
import { useMonitoring } from "@/context/MonitoringContext";
import { useRemoteReport } from "@/hooks/useRemoteReport";
import { parseAnomalies } from "@/lib/anomalies";
import { formatMs } from "@/lib/analytics";

type Cursor = { before: string; to: string };
const date = (value: string) => new Date(value).toLocaleString();

export default function AnomalyFeed() {
  const { anomalies, anomaliesStale, range } = useMonitoring();
  const [type, setType] = useState("");
  const [pagination, setPagination] = useState<{ key: string; pages: Cursor[] }>({ key: "", pages: [] });
  const key = `${range}:${type}`;
  const pages = pagination.key === key ? pagination.pages : [];
  const cursor = pages.at(-1);
  const parameters = new URLSearchParams({ range, limit: "50" });
  if (type) parameters.set("type",type);
  if (cursor) { parameters.set("before",cursor.before); parameters.set("to",cursor.to) }
  const remote = useRemoteReport(type || cursor ? `/api/anomalies?${parameters}` : null,parseAnomalies,15_000,0);
  const state = type || cursor ? remote : anomalies;
  const report = state.data;
  return (
    <section className="space-y-4 rounded-xl border border-white/[0.07] bg-[#111820] p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div><h2 className="text-lg font-medium">Latency anomaly feed</h2><p className="mt-1 max-w-3xl text-xs leading-5 text-gray-400">Rolling median/MAD high-latency episodes overlapping the global range, newest onset first. Repeated high probes update one episode. Failed or missing probes do not establish recovery or identify a cause.</p></div>
        <label className="text-xs text-gray-300">Probe type <select aria-label="Anomaly feed probe type" value={type} onChange={(e) => setType(e.target.value)} className="ml-2 rounded-lg border border-white/10 bg-[#0B1016] p-2"><option value="">All</option><option value="tcp">TCP</option><option value="dns">DNS</option></select></label>
      </div>
      {state.loading && <p role="status" className="text-sm text-gray-400">Loading anomaly episodes...</p>}
      {state.error && <p role="alert" className="text-sm text-amber-300">{state.error}</p>}
      {!type && !cursor && anomaliesStale && <p role="status" className="text-sm text-amber-300">The anomaly feed is stale. Check its window end and latest observation timestamps.</p>}
      {report && <p className="text-xs text-gray-400">Page {pages.length+1} · {report.count} episodes · window ends {date(report.as_of)}. Episode status reflects the latest stored update.</p>}
      {report && report.count === 0 && <p className="py-4 text-sm text-gray-400">{cursor ? "No older episodes remain on this page. Return to the latest page to refresh." : "No recorded latency episodes overlap this selection. Detection needs enough recent normal successes before it can flag a target."}</p>}
      <ol className="space-y-3" aria-label="Latency anomaly episodes">
        {report?.anomalies.map((e) => <li key={e.id} className="rounded-lg border border-white/[0.08] bg-[#0B1016] p-4">
          <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="min-w-0 break-all text-sm font-medium">{e.type.toUpperCase()} · {e.target}</h3><div className="flex gap-3 text-xs"><span className={e.severity === "critical" ? "text-red-300" : "text-amber-300"}>{e.severity}</span><span className={e.recovered_at ? "text-teal-300" : "text-red-300"}>{e.recovered_at ? "Recovered" : "Open"}</span></div></div>
          <p className="mt-2 text-xs leading-6 text-gray-300">Onset {date(e.timestamp)} · last high measurement {date(e.last_anomaly_at)} · last observed probe {date(e.last_observed_at)}{e.recovered_at ? ` · recovered ${date(e.recovered_at)}` : ""}</p>
          <dl className="mt-3 grid gap-3 text-xs sm:grid-cols-2 xl:grid-cols-5">{[["Onset latency",e.observed_latency_ms],["Peak latency",e.peak_latency_ms],["Baseline median",e.baseline_ms],["Baseline MAD",e.mad_ms],["Frozen threshold",e.threshold_ms]].map(([label,value]) => <div key={String(label)}><dt className="text-gray-400">{label}</dt><dd className="mt-1 text-gray-200">{formatMs(Number(value))}</dd></div>)}</dl>
          <details className="mt-3 text-xs text-gray-400"><summary className="cursor-pointer">Method and recovery policy</summary><p className="mt-2 leading-6">{e.method}. Threshold = median + max({e.policy.min_delta_ms} ms, {e.policy.mad_multiplier} × MAD), with {e.baseline_samples} preceding normal successes (window {e.policy.window}, minimum {e.policy.min_samples}, maximum age {e.policy.max_age_ns/60e9} minutes). {e.anomalous_samples} high measurements belong to this episode. Recovery requires {e.policy.recovery_samples} consecutive normal-range successes. An open episode keeps its original threshold across restarts.</p></details>
        </li>)}
      </ol>
      <div className="flex flex-wrap gap-3 text-xs">
        <button type="button" disabled={pages.length === 0 || state.loading} onClick={() => setPagination({ key, pages:[] })} className="rounded-lg border border-white/10 px-3 py-2 disabled:opacity-40">Latest episodes</button>
        <button type="button" disabled={pages.length === 0 || state.loading} onClick={() => setPagination({ key, pages:pages.slice(0,-1) })} className="rounded-lg border border-white/10 px-3 py-2 disabled:opacity-40">Previous page</button>
        <button type="button" disabled={!report?.has_more || state.loading} onClick={() => { if (report?.has_more) setPagination({ key, pages:[...pages,{ before:report.next_cursor,to:cursor?.to ?? report.as_of }] }) }} className="rounded-lg border border-teal-500/30 px-3 py-2 text-teal-300 disabled:opacity-40">Older episodes</button>
      </div>
    </section>
  );
}
