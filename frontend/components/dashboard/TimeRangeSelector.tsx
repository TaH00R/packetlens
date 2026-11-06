
"use client";

import { useMonitoring } from "@/context/MonitoringContext";
import type { TimeRange } from "@/context/MonitoringContext";

const OPTIONS: { label: string; value: TimeRange }[] = [
  { label: "5m", value: "5m" },
  { label: "15m", value: "15m" },
  { label: "1h", value: "1h" },
  { label: "6h", value: "6h" },
  { label: "24h", value: "24h" },
  { label: "All", value: "all" },
];

export default function TimeRangeSelector() {
  const { range, setRange } = useMonitoring();

  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p className="text-sm font-medium text-white">
          Time range
        </p>
        <p className="text-xs text-gray-500">
          Showing up to 1,000 recent cycles; longer ranges may be incomplete
        </p>
      </div>

      <div
        className="flex flex-wrap items-center gap-1 rounded-lg border border-white/[0.08] bg-[#111820] p-1"
        role="group"
        aria-label="Select time range"
      >
        {OPTIONS.map((option) => {
          const active = range === option.value;

          return (
            <button
              key={option.value}
              type="button"
              onClick={() => setRange(option.value)}
              aria-pressed={active}
              className={`rounded-md px-3 py-2 text-xs font-medium transition-colors ${
                active
                  ? "bg-teal-500/15 text-teal-400"
                  : "text-gray-500 hover:bg-white/[0.04] hover:text-white"
              }`}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
