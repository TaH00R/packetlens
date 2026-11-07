"use client";
import { useState } from "react";
import type { ProbeResult } from "@/types/metrics";
interface Row extends ProbeResult {
  resolver?: string;
  outcome?: string;
}
export default function ActivityTable({ results }: { results: Row[] }) {
  const [type, setType] = useState("");
  const [page, setPage] = useState(0);
  const rows = results
    .filter((row) => !type || row.type === type)
    .slice()
    .sort((a, b) => Date.parse(b.timestamp) - Date.parse(a.timestamp));
  const lastPage = Math.max(0, Math.ceil(rows.length / 50) - 1);
  const current = Math.min(page, lastPage);
  const visible = rows.slice(current * 50, current * 50 + 50);
  return (
    <section className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs leading-6 text-slate-400">
          {rows.length} observations in the latest 1,000 cached cycles within
          the global range. Durations include failed attempt time.
        </p>
        <label className="text-xs text-slate-300">
          Probe type{" "}
          <select
            aria-label="Activity probe type"
            className="ml-2 rounded-lg border border-white/10 bg-[#111820] p-2"
            value={type}
            onChange={(event) => {
              setType(event.target.value);
              setPage(0);
            }}
          >
            <option value="">All</option>
            <option value="tcp">TCP</option>
            <option value="dns">DNS</option>
          </select>
        </label>
      </div>
      <div className="overflow-x-auto rounded-xl border border-white/[0.07] bg-[#111820]">
        <table className="w-full text-left text-xs">
          <thead>
            <tr>
              {[
                "Timestamp",
                "Probe",
                "Target",
                "Resolver",
                "Duration",
                "Status",
                "Outcome",
                "Error",
              ].map((label) => (
                <th
                  scope="col"
                  key={label}
                  className="whitespace-nowrap px-4 py-3 font-medium text-slate-400"
                >
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((row, index) => (
              <tr
                key={`${row.timestamp}-${row.target}-${row.resolver ?? "system"}-${index}`}
                className="border-t border-white/5 text-slate-300"
              >
                <td className="whitespace-nowrap p-4">
                  {new Date(row.timestamp).toLocaleString()}
                </td>
                <td className="p-4 uppercase">{row.type}</td>
                <th scope="row" className="max-w-72 break-all p-4 font-medium">
                  {row.target}
                </th>
                <td className="whitespace-nowrap p-4">
                  {row.resolver ?? (row.type === "dns" ? "System" : "—")}
                </td>
                <td className="whitespace-nowrap p-4 tabular-nums">
                  {row.latency_ms.toFixed(2)} ms
                </td>
                <td className="p-4">
                  <span
                    className={`rounded-md px-2 py-1 ${row.success ? "bg-teal-500/10 text-teal-300" : "bg-red-500/10 text-red-300"}`}
                  >
                    {row.success ? "Success" : "Failed"}
                  </span>
                </td>
                <td className="whitespace-nowrap p-4">{row.outcome ?? "—"}</td>
                <td className="min-w-48 max-w-sm break-words p-4 text-slate-400">
                  {row.error || "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!rows.length && (
          <p className="p-6 text-sm text-slate-400">
            No probe activity matches this selection. Missing measurements do
            not indicate success.
          </p>
        )}
      </div>
      <div className="flex items-center gap-3 text-xs text-slate-400">
        <button
          type="button"
          className="rounded-lg border border-white/10 px-3 py-2 disabled:opacity-40"
          disabled={current === 0}
          onClick={() => setPage(current - 1)}
        >
          Newer activity
        </button>
        <span>
          Page {current + 1} of {lastPage + 1}
        </span>
        <button
          type="button"
          className="rounded-lg border border-white/10 px-3 py-2 disabled:opacity-40"
          disabled={current === lastPage}
          onClick={() => setPage(current + 1)}
        >
          Older activity
        </button>
      </div>
    </section>
  );
}
