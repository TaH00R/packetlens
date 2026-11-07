"use client";

import { useState } from "react";
import { useMonitoring } from "@/context/MonitoringContext";
import { useEpisodeFeed } from "@/hooks/useEpisodeFeed";
import { parseCorrelations } from "@/lib/correlations";
import EpisodePagination from "./EpisodePagination";

const date = (value: string) => new Date(value).toLocaleString();

export default function CorrelationFeed() {
  const { correlations, correlationsStale, range } = useMonitoring();
  const [type, setType] = useState("");
  const { state, pageNumber, hasPrevious, isLatest, latest, previous, next } = useEpisodeFeed("/api/correlations", parseCorrelations, correlations, range, { type });
  const report = state.data;
  return <section className="space-y-4" aria-label="Cross-target failure correlation">
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div><h2 className="text-xl font-medium">Possible shared connectivity issues</h2><p className="mt-2 max-w-3xl text-xs leading-6 text-gray-400">Recent failures across distinct targets can suggest a broader issue. Correlation is not a confirmed root cause and can include isolated failures. Target incidents below track sustained failures and confirmed recovery separately.</p></div>
      <label className="text-xs text-gray-300">Affected probe <select aria-label="Correlation probe type" value={type} onChange={e => setType(e.target.value)} className="ml-2 rounded-lg border border-white/10 bg-[#111820] p-2"><option value="">All</option><option value="tcp">TCP</option><option value="dns">DNS</option></select></label>
    </div>
    {state.loading && <p role="status" className="text-sm text-gray-400">Loading correlated failures...</p>}
    {state.error && <p role="alert" className="text-sm text-amber-300">{state.error}</p>}
    {!type && isLatest && correlationsStale && <p role="status" className="text-sm text-amber-300">This correlation report is stale. Current connectivity is unknown.</p>}
    {report && <p className="text-xs text-gray-400">Page {pageNumber} · {report.count} correlation events · window ends {date(report.as_of)}. Newest first; status reflects the latest stored evaluation.</p>}
    {report?.count === 0 && <p className="rounded-xl border border-white/10 bg-[#111820] p-5 text-sm text-gray-400">No overlapping cross-target failures match this selection. Missing measurements do not establish healthy connectivity.</p>}
    <ol className="space-y-3" aria-label="Correlated failure timeline">{report?.correlations.map(event => <li key={event.id} className="rounded-xl border border-blue-500/20 bg-[#111820] p-5">
      <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="font-medium text-blue-200">Possible shared connectivity issue</h3><span className="rounded-md bg-blue-500/10 px-2 py-1 text-xs text-blue-200">{event.ended_at ? "Overlap ended" : "Overlap observed"}</span></div>
      <p className="mt-3 text-xs leading-6 text-gray-300">Detected {date(event.detected_at)}. Last overlap observed {date(event.last_correlated_at)}. Policy: at least {event.policy.min_targets} distinct targets within {event.policy.window_ns / 1e9} seconds.</p>
      <ul aria-label="Affected targets" className="mt-3 flex flex-wrap gap-2">{event.members.map(member => <li key={`${member.type}:${member.target}`} className="max-w-full break-all rounded-md bg-white/5 px-2 py-1 text-xs text-gray-300">{member.type.toUpperCase()} · {member.target}</li>)}</ul>
      <p className="mt-3 text-xs leading-6 text-gray-400">Members include all targets correlated during this episode; they need not all fail simultaneously. No confirmed cause is known.</p>
      {event.ended_at ? <p className="mt-2 text-xs leading-6 text-gray-400">Overlap ended at evaluation {date(event.ended_at)}. This can reflect successes or expired observations; it does not confirm recovery for every target.</p> : Date.parse(report.as_of) - Date.parse(event.last_correlated_at) > event.policy.window_ns / 1e6 && <p role="status" className="mt-2 text-xs text-amber-300">No recent overlap evaluation is available. Current connectivity is unknown; measurements must resume before this event can be updated.</p>}
    </li>)}</ol>
    <EpisodePagination hasPrevious={hasPrevious} hasMore={report?.has_more ?? false} loading={state.loading} latest={latest} previous={previous} next={next} noun="correlations" />
  </section>;
}
