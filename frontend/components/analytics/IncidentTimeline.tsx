"use client";

import { useState } from "react";
import { Activity, ShieldCheck } from "lucide-react";
import { useMonitoring } from "@/context/MonitoringContext";
import { useEpisodeFeed } from "@/hooks/useEpisodeFeed";
import { formatIncidentDuration, parseIncidents } from "@/lib/incidents";
import MetricCard from "@/components/dashboard/MetricCard";
import EpisodePagination from "./EpisodePagination";

const date = (value: string) => new Date(value).toLocaleString();

export default function IncidentTimeline() {
  const { incidents, incidentsStale, range } = useMonitoring();
  const [type, setType] = useState("");
  const [status, setStatus] = useState("all");
  const { state, pageNumber, hasPrevious, isLatest, latest, previous, next } = useEpisodeFeed("/api/incidents",parseIncidents,incidents,range,{ type, status: status === "all" ? "" : status });
  const report = state.data;
  return (
    <section className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div><h2 className="text-xl font-medium">Failure incident timeline</h2><p className="mt-2 max-w-3xl text-xs leading-6 text-gray-400">Incidents require sustained observed failures for one probe type and target. Isolated failed probes remain in recent activity. Recovery requires successful confirmation. Missing measurements are unknown, and these observations do not establish a root cause.</p></div>
        <div className="flex flex-wrap gap-3 text-xs text-gray-300">
          <label>Probe type <select aria-label="Incident probe type" value={type} onChange={(e) => setType(e.target.value)} className="ml-2 rounded-lg border border-white/10 bg-[#111820] p-2"><option value="">All</option><option value="tcp">TCP</option><option value="dns">DNS</option></select></label>
          <label>Status <select aria-label="Incident status" value={status} onChange={(e) => setStatus(e.target.value)} className="ml-2 rounded-lg border border-white/10 bg-[#111820] p-2"><option value="all">All</option><option value="open">Open</option><option value="recovered">Recovered</option></select></label>
        </div>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <MetricCard title="Open incidents in selection" value={report ? String(report.open_count) : "No data"} description="All pages and statuses; current range and probe filter" icon={Activity} color="text-red-300" />
        <MetricCard title="Recovered incidents in selection" value={report ? String(report.recovered_count) : "No data"} description="All pages and statuses; current range and probe filter" icon={ShieldCheck} color="text-teal-300" />
      </div>
      {state.loading && <p role="status" className="text-sm text-gray-400">Loading failure incidents...</p>}
      {state.error && <p role="alert" className="text-sm text-amber-300">{state.error}</p>}
      {!type && status === "all" && isLatest && incidentsStale && <p role="status" className="text-sm text-amber-300">This incident report is stale. Check the last observed probe timestamp before interpreting its status.</p>}
      {report && <p className="text-xs text-gray-400">Page {pageNumber} · {report.count} incidents · window ends {date(report.as_of)}. Sorted by first failure, newest first. Status reflects the latest stored update. Older open incidents overlap the selection.</p>}
      {report && report.count === 0 && <p className="rounded-xl border border-white/10 bg-[#111820] p-5 text-sm text-gray-400">{hasPrevious ? "No older incidents remain on this page. Return to the latest page to refresh." : "No recorded incidents match this selection. Isolated failures, incomplete streaks, and unmeasured time do not establish a sustained incident."}</p>}
      <ol className="space-y-4" aria-label="Failure incident timeline">
        {report?.incidents.map((e) => <li key={e.id} className="rounded-xl border border-white/[0.08] bg-[#111820] p-5">
          <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="min-w-0 break-all font-medium">{e.type.toUpperCase()} · {e.target}</h3><span className={`rounded-md px-2 py-1 text-xs ${e.recovered_at ? "bg-teal-500/10 text-teal-300" : "bg-red-500/10 text-red-300"}`}>{e.recovered_at ? "Recovered incident" : "Ongoing incident"}</span></div>
          <ol className="mt-4 space-y-2 border-l border-white/10 pl-4 text-xs leading-6 text-gray-300" aria-label={`Lifecycle for ${e.target}`}>
            <li>First failed probe: {date(e.started_at)}</li>
            <li>Incident detected: {date(e.detected_at)} after {e.policy.failure_threshold} consecutive observed failures.</li>
            <li>Last failed probe: {date(e.last_failure_at)} · {e.failed_probes} failures observed in this episode.</li>
            <li>Last observed probe: {date(e.last_observed_at)}</li>
            <li>{e.recovered_at ? `Recovery confirmed: ${date(e.recovered_at)}. Elapsed episode span: ${formatIncidentDuration(e.duration_ms)}.` : `Recovery confirmation: ${e.recovery_successes} / ${e.policy.recovery_threshold} consecutive successes.`}</li>
          </ol>
          {e.has_monitoring_gaps && <p className="mt-3 text-xs leading-5 text-amber-300">This episode contains observation gaps. Its elapsed span includes unmeasured time and does not prove continuous downtime.</p>}
          {!e.recovered_at && Date.parse(report.as_of)-Date.parse(e.last_observed_at)>e.policy.max_gap_ns/1e6 && <p className="mt-3 text-xs leading-5 text-amber-300">No recent probe observation is available. Current target state is unknown; the incident awaits recovery confirmation.</p>}
          <details className="mt-4 text-xs text-gray-400"><summary className="cursor-pointer">Probe errors and policy</summary><p className="mt-2 break-words leading-6">First error: {e.first_error || "No detail recorded"}. Last error: {e.last_error || "No detail recorded"}. Failure threshold {e.policy.failure_threshold}; recovery threshold {e.policy.recovery_threshold}; maximum gap {e.policy.max_gap_ns/1e9} seconds. Policy is fixed for the failure run across restarts. Elapsed duration runs from the first failed observation to confirmed recovery.</p></details>
        </li>)}
      </ol>
      <EpisodePagination hasPrevious={hasPrevious} hasMore={report?.has_more ?? false} loading={state.loading} latest={latest} previous={previous} next={next} noun="incidents" />
    </section>
  );
}
