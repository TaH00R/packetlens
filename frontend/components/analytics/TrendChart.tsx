"use client";

import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { AnalyticsPoint } from "@/types/analytics";

interface Props { title: string; description: string; points: AnalyticsPoint[]; metric: "avg_latency_ms" | "jitter_ms" | "availability" }
export default function TrendChart({ title, description, points, metric }: Props) {
  const percent = metric === "availability";
  const hasData = points.some((point) => point[metric] !== null);
  const data = points.map((point) => ({ ...point, timestamp: Date.parse(point.timestamp) }));
  return (
    <section className="min-w-0 rounded-xl border border-white/[0.07] bg-[#111820] p-5">
      <h3 className="font-medium">{title}</h3>
      <p className="mb-5 mt-1 text-xs leading-5 text-gray-400">{description}</p>
      {!hasData ? <p className="flex h-[250px] items-center justify-center text-sm text-gray-400">No comparable measurements in this window.</p> : (
        <div className="h-[250px] w-full" role="img" aria-label={`${title}. ${points.reduce((sum, p) => sum+p.attempts, 0)} observed probes; gaps indicate missing data.`}>
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top: 10, right: 15, left: 0, bottom: 0 }}>
              <CartesianGrid stroke="#26313d" strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="timestamp" type="number" domain={["dataMin", "dataMax"]} scale="time" tickFormatter={(v) => new Date(v).toLocaleString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })} tick={{ fill: "#94a3b8", fontSize: 10 }} minTickGap={45} />
              <YAxis domain={percent ? [0, 100] : [0, "auto"]} width={65} tick={{ fill: "#94a3b8", fontSize: 10 }} tickFormatter={(v) => `${v}${percent ? "%" : " ms"}`} />
              <Tooltip contentStyle={{ background: "#111820", border: "1px solid #26313d", borderRadius: 10 }} labelFormatter={(v) => new Date(Number(v)).toLocaleString()} formatter={(v) => [v == null ? "No data" : `${Number(v).toFixed(2)}${percent ? "%" : " ms"}`, title]} />
              <Line dataKey={metric} name={title} type="monotone" stroke={metric === "jitter_ms" ? "#3B82F6" : "#14B8A6"} strokeWidth={2} dot={{ r: 2 }} connectNulls={false} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </section>
  );
}
