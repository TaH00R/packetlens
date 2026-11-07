"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  chartGrid,
  chartTick,
  chartTime,
  chartTooltipStyle,
  chartValue,
} from "@/lib/chart-style";
import type { AnalyticsPoint } from "@/types/analytics";

interface Props {
  title: string;
  description: string;
  points: AnalyticsPoint[];
  metric: "avg_latency_ms" | "jitter_ms" | "availability";
}
export default function TrendChart({
  title,
  description,
  points,
  metric,
}: Props) {
  const percent = metric === "availability";
  const showAnomalies =
    metric === "avg_latency_ms" &&
    points.some((p) => (p.anomaly_count ?? 0) > 0);
  const hasData =
    showAnomalies || points.some((point) => point[metric] !== null);
  const data = points.map((point) => ({
    ...point,
    timestamp: Date.parse(point.timestamp),
  }));
  return (
    <section className="min-w-0 rounded-xl border border-white/[0.07] bg-[#111820] p-5">
      <h3 className="font-medium">{title}</h3>
      <p className="mb-5 mt-1 text-xs leading-5 text-gray-400">{description}</p>
      {showAnomalies && (
        <p className="mb-3 text-xs text-red-300">
          Red markers: maximum anomaly-onset latency per bucket. Every onset is
          counted, including when multiple episodes share a bucket.
        </p>
      )}
      {!hasData ? (
        <p className="flex h-[290px] items-center justify-center text-sm text-gray-400">
          No comparable measurements in this window.
        </p>
      ) : (
        <div
          className="h-[290px] w-full"
          role="img"
          aria-label={`${title}. ${points.reduce((sum, p) => sum + p.attempts, 0)} observed probes; gaps indicate missing data.`}
        >
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={data}
              margin={{ top: 10, right: 15, left: 0, bottom: 0 }}
            >
              <CartesianGrid {...chartGrid} />
              <XAxis
                dataKey="timestamp"
                type="number"
                domain={["dataMin", "dataMax"]}
                scale="time"
                tickFormatter={chartTime}
                tick={chartTick}
                axisLine={false}
                tickLine={false}
                minTickGap={75}
                tickMargin={12}
              />
              <YAxis
                domain={percent ? [0, 100] : [0, "auto"]}
                width={70}
                tick={chartTick}
                axisLine={false}
                tickLine={false}
                tickMargin={8}
                tickFormatter={(v) =>
                  `${chartValue(v)}${percent ? "%" : " ms"}`
                }
              />
              <Tooltip
                contentStyle={chartTooltipStyle}
                cursor={{ stroke: "#64748b", strokeDasharray: "4 4" }}
                labelFormatter={(v) => new Date(Number(v)).toLocaleString()}
                formatter={(v, name, item) => [
                  v == null
                    ? "No data"
                    : `${Number(v).toFixed(2)}${percent ? "%" : " ms"}`,
                  name === "Anomaly onset"
                    ? `Anomaly onset (${Number(item.payload.anomaly_count)} episodes)`
                    : title,
                ]}
              />
              <Line
                dataKey={metric}
                name={title}
                type="monotone"
                stroke={metric === "jitter_ms" ? "#3B82F6" : "#14B8A6"}
                strokeWidth={2.5}
                strokeLinecap="round"
                dot={
                  points.filter((p) => p[metric] !== null).length <= 3
                    ? { r: 3, strokeWidth: 0 }
                    : false
                }
                activeDot={{ r: 5, stroke: "#111820", strokeWidth: 2 }}
                connectNulls={false}
                isAnimationActive={false}
              />
              {showAnomalies && (
                <Line
                  dataKey="anomaly_latency_ms"
                  name="Anomaly onset"
                  stroke="none"
                  dot={{ r: 4, fill: "#F87171", stroke: "#F87171" }}
                  connectNulls={false}
                  isAnimationActive={false}
                />
              )}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </section>
  );
}
