"use client";

import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { HistogramBucket } from "@/types/analytics";

export default function DistributionChart({ bins }: { bins: HistogramBucket[] }) {
  const total = bins.reduce((sum, bin) => sum+bin.count, 0);
  const data = bins.map((b) => ({ ...b, interval: b.upper_ms === null ? `${b.lower_ms}+` : `${b.lower_ms}-${b.upper_ms}` }));
  return (
    <section className="min-w-0 rounded-xl border border-white/[0.07] bg-[#111820] p-5">
      <h3 className="font-medium">Latency distribution</h3>
      <p className="mb-5 mt-1 text-xs text-gray-400">Successful probes only. Bins include the lower bound and exclude the upper bound, in ms.</p>
      {total === 0 ? <p className="flex h-[250px] items-center justify-center text-sm text-gray-400">No successful latency measurements in this window.</p> : (
        <div className="h-[250px]" role="img" aria-label={`Latency distribution of ${total} successful probes`}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data}>
              <CartesianGrid stroke="#26313d" vertical={false} />
              <XAxis dataKey="interval" tick={{ fill: "#94a3b8", fontSize: 10 }} />
              <YAxis allowDecimals={false} width={45} tick={{ fill: "#94a3b8", fontSize: 10 }} />
              <Tooltip contentStyle={{ background: "#111820", border: "1px solid #26313d", borderRadius: 10 }} labelFormatter={(label) => `${label} ms`} formatter={(v) => [v, "Successful probes"]} />
              <Bar dataKey="count" fill="#14B8A6" radius={[4, 4, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </section>
  );
}
