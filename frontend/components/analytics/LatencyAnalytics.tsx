"use client";
import { useState } from "react";
import { Activity, Gauge } from "lucide-react";
import { useMonitoring } from "@/context/MonitoringContext";
import { formatMs, formatRate, percentChange } from "@/lib/analytics";
import type { ProbeType } from "@/types/analytics";
import MetricCard from "@/components/dashboard/MetricCard";
import DataTabs from "@/components/dashboard/DataTabs";
import DistributionChart from "./DistributionChart";
import TrendChart from "./TrendChart";

export default function LatencyAnalytics({ type }: { type: ProbeType }) {
  const { analytics, analyticsStale } = useMonitoring();
  const [target, setTarget] = useState("");
  const report = analytics.data;
  const targets = report?.current.targets.filter((t) => t.type === type) ?? [];
  const selected = targets.some((t) => t.target === target) ? target : "";
  const summary = selected
    ? targets.find((t) => t.target === selected)
    : report?.current.groups.find((g) => g.type === type);
  const previous = selected
    ? report?.previous?.targets.find(
        (t) => t.type === type && t.target === selected,
      )
    : report?.previous?.groups.find((g) => g.type === type);
  const points =
    (selected
      ? report?.current.target_trends.find(
          (s) => s.type === type && s.target === selected,
        )
      : report?.current.trends.find((s) => s.type === type)
    )?.points ?? [];
  const latency = (
    <div className="space-y-5">
      <div className="grid gap-4 sm:grid-cols-3">
        {[
          ["Average latency", summary?.avg_latency_ms],
          ["Minimum latency", summary?.min_latency_ms],
          ["Maximum latency", summary?.max_latency_ms],
        ].map(([title, value]) => (
          <MetricCard
            key={String(title)}
            title={String(title)}
            value={formatMs(typeof value === "number" ? value : null)}
            description="Successful measurements in selection"
            icon={Activity}
            color="text-teal-400"
          />
        ))}
      </div>
      <TrendChart
        title="Latency trend"
        description="Mean successful latency per time bucket. Missing and all-failed buckets stay empty."
        points={points}
        metric="avg_latency_ms"
      />
    </div>
  );
  const percentiles = (
    <div className="space-y-5">
      <p className="text-xs leading-6 text-gray-400">
        Exact nearest ranks over successful measurements. No interpolation or
        failed-probe zeros.
      </p>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {([50, 90, 95, 99] as const).map((p) => (
          <MetricCard
            key={p}
            title={`P${p} latency`}
            value={formatMs(summary?.[`p${p}_ms`])}
            description="Nearest-rank successful latency"
            icon={Gauge}
            color="text-teal-400"
          />
        ))}
      </div>
    </div>
  );
  const jitter = (
    <div className="space-y-5">
      <div className="grid gap-4 sm:grid-cols-2">
        <MetricCard
          title="Mean jitter"
          value={formatMs(summary?.jitter_ms)}
          description="Mean per-target successful latency variation"
          icon={Activity}
          color="text-blue-400"
        />
        <MetricCard
          title="Latest pair"
          value={formatMs(summary?.current_jitter_ms)}
          description={
            selected
              ? "Latest successive-success difference"
              : "Select a target to inspect its latest pair"
          }
          icon={Activity}
          color="text-blue-400"
        />
      </div>
      <TrendChart
        title="Jitter trend"
        description="Absolute differences between consecutive successes, calculated per target. Pairs may span failures; missing pairs stay empty."
        points={points}
        metric="jitter_ms"
      />
    </div>
  );
  const targetTable = (
    <section className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#111820]">
      <h3 className="p-5 font-medium">Target comparison</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead>
            <tr>
              {[
                "Target",
                "Attempts",
                "Success",
                "Minimum",
                "Average",
                "Maximum",
                "P50",
                "P90",
                "P95",
                "P99",
                "Mean jitter",
                "Latest pair",
              ].map((label) => (
                <th
                  scope="col"
                  className="whitespace-nowrap px-4 py-3 font-medium text-gray-400"
                  key={label}
                >
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {targets.map((t) => (
              <tr
                className="border-t border-white/5 text-gray-300"
                key={t.target}
              >
                <th scope="row" className="whitespace-nowrap p-4 font-medium">
                  {t.target}
                </th>
                <td className="p-4">{t.attempts}</td>
                <td className="whitespace-nowrap p-4">
                  {formatRate(t.availability)}
                </td>
                {[
                  t.min_latency_ms,
                  t.avg_latency_ms,
                  t.max_latency_ms,
                  t.p50_ms,
                  t.p90_ms,
                  t.p95_ms,
                  t.p99_ms,
                  t.jitter_ms,
                  t.current_jitter_ms,
                ].map((value, i) => (
                  <td key={i} className="whitespace-nowrap p-4">
                    {formatMs(value)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {targets.length === 0 && (
        <p className="p-5 text-sm text-gray-400">
          No targets observed in the selected range.
        </p>
      )}
    </section>
  );
  const comparison = (
    <section className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#111820]">
      <h3 className="p-5 font-medium">Previous-period comparison</h3>
      {report?.previous ? (
        <>
          <p className="px-5 pb-4 text-xs leading-6 text-gray-400">
            Previous period: {new Date(report.previous.from).toLocaleString()}{" "}
            to {new Date(report.previous.to).toLocaleString()} (
            {previous?.attempts ?? 0} attempts).
          </p>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr>
                  {["Metric", "Current", "Previous", "Change"].map((label) => (
                    <th
                      scope="col"
                      key={label}
                      className="px-5 py-3 text-xs font-medium text-gray-400"
                    >
                      {label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {(
                  [
                    ["Average", "avg_latency_ms"],
                    ["Minimum", "min_latency_ms"],
                    ["Maximum", "max_latency_ms"],
                    ["P50", "p50_ms"],
                    ["P90", "p90_ms"],
                    ["P95", "p95_ms"],
                    ["P99", "p99_ms"],
                    ["Jitter", "jitter_ms"],
                  ] as const
                ).map(([label, key]) => (
                  <tr
                    key={key}
                    className="border-t border-white/5 text-gray-300"
                  >
                    <th scope="row" className="p-5 font-medium">
                      {label}
                    </th>
                    <td className="whitespace-nowrap p-5">
                      {formatMs(summary?.[key])}
                    </td>
                    <td className="whitespace-nowrap p-5">
                      {formatMs(previous?.[key])}
                    </td>
                    <td className="whitespace-nowrap p-5">
                      {percentChange(summary?.[key], previous?.[key])}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : (
        <p className="p-5 text-sm text-gray-400">
          Select a finite time range to compare it with the preceding
          equal-length period.
        </p>
      )}
    </section>
  );
  return (
    <div className="min-w-0 space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="text-lg font-medium">
            Stored {type.toUpperCase()} performance
          </h2>
          <p className="mt-1 text-xs leading-6 text-gray-400">
            All retained measurements in the selected range.
            {report
              ? ` As of ${new Date(report.as_of).toLocaleString()} \u00b7 ${summary?.attempts ?? 0} observed attempts.`
              : ""}
          </p>
        </div>
        <label className="flex min-w-0 max-w-full flex-wrap items-center gap-2 text-xs text-gray-300">
          Target
          <select
            aria-label={`${type.toUpperCase()} analytics target`}
            className="min-w-0 max-w-full rounded-lg border border-white/10 bg-[#111820] p-2 text-sm"
            value={selected}
            onChange={(e) => setTarget(e.target.value)}
            disabled={!report || targets.length === 0}
          >
            <option value="">All {type.toUpperCase()} targets</option>
            {targets.map((t) => (
              <option key={t.target} value={t.target}>
                {t.target}
              </option>
            ))}
          </select>
        </label>
      </div>
      {analytics.loading && (
        <p role="status" className="text-sm text-gray-400">
          {report
            ? "Refreshing stored analytics..."
            : "Loading stored analytics..."}
        </p>
      )}
      {analyticsStale && (
        <p role="status" className="text-sm text-amber-300">
          This stored report is stale. Check its observation time before
          interpreting the results.
        </p>
      )}
      {analytics.error && (
        <p
          role="alert"
          className="rounded-lg border border-amber-500/20 p-3 text-sm text-amber-300"
        >
          {analytics.error}
        </p>
      )}
      {report?.history_start &&
        Date.parse(report.history_start) > Date.parse(report.current.from) && (
          <p className="text-xs text-amber-300">
            Retained history begins{" "}
            {new Date(report.history_start).toLocaleString()}. Earlier parts of
            this selection have no retained data.
          </p>
        )}
      {!report && !analytics.loading ? (
        <p className="text-sm text-gray-400">
          Stored analytics are unavailable. Recent live activity is available on
          the probe activity page.
        </p>
      ) : (
        <DataTabs
          label={`${type.toUpperCase()} analytics views`}
          tabs={[
            { id: "latency", label: "Latency", content: latency },
            { id: "percentiles", label: "Percentiles", content: percentiles },
            {
              id: "distribution",
              label: "Distribution",
              content: <DistributionChart bins={summary?.histogram ?? []} />,
            },
            { id: "jitter", label: "Jitter", content: jitter },
            { id: "targets", label: "Targets", content: targetTable },
            {
              id: "comparison",
              label: "Period comparison",
              content: comparison,
            },
          ]}
        />
      )}
    </div>
  );
}
