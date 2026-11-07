"use client";

import { chartGrid, chartTime, chartTooltipStyle } from "@/lib/chart-style";
import { useMemo } from "react";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type {
  LatencyPoint,
  MonitoringSnapshot,
  ProbeResult,
} from "@/types/metrics";

interface LatencyChartProps {
  results: MonitoringSnapshot[];
}

function calculateAverage(results: ProbeResult[]): number | null {
  const successful = results.filter((result) => result.success);

  if (successful.length === 0) return null;

  const total = successful.reduce((sum, result) => sum + result.latency_ms, 0);

  return Number((total / successful.length).toFixed(2));
}

export default function LatencyChart({ results }: LatencyChartProps) {
  const chartData = useMemo<LatencyPoint[]>(() => {
    return results.map((snapshot) => ({
      timestamp: new Date(snapshot.timestamp).getTime(),
      tcp: calculateAverage(snapshot.tcp),
      dns: calculateAverage(snapshot.dns),
    }));
  }, [results]);

  if (!chartData.some((point) => point.tcp !== null || point.dns !== null)) {
    return (
      <p className="flex h-[320px] items-center justify-center text-sm text-gray-400">
        No measurements in the selected time range.
      </p>
    );
  }

  return (
    <div className="h-[320px] w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={chartData}
          margin={{ top: 10, right: 20, left: 0, bottom: 0 }}
        >
          <CartesianGrid {...chartGrid} />

          <XAxis
            dataKey="timestamp"
            type="number"
            scale="time"
            domain={["dataMin", "dataMax"]}
            tickFormatter={chartTime}
            tick={{ fill: "#8492a6", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            minTickGap={75}
            tickMargin={10}
          />

          <YAxis
            tick={{ fill: "#8492a6", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            width={65}
            tickFormatter={(value) => `${value} ms`}
          />

          <Tooltip
            contentStyle={chartTooltipStyle}
            labelStyle={{ color: "#94a3b8" }}
            labelFormatter={(value) =>
              new Date(Number(value)).toLocaleTimeString()
            }
            formatter={(value, name) => [
              value == null ? "No measurement" : `${value} ms`,
              name === "tcp" ? "TCP latency" : "DNS latency",
            ]}
          />

          <Line
            type="monotone"
            dataKey="tcp"
            name="tcp"
            stroke="#14B8A6"
            strokeWidth={2.5}
            strokeLinecap="round"
            dot={
              chartData.filter((point) => point.tcp !== null).length <= 3
                ? { r: 3, strokeWidth: 0 }
                : false
            }
            activeDot={{ r: 5, stroke: "#111820", strokeWidth: 2 }}
            connectNulls={false}
            isAnimationActive={false}
          />

          <Line
            type="monotone"
            dataKey="dns"
            name="dns"
            stroke="#3B82F6"
            strokeWidth={2.5}
            strokeLinecap="round"
            dot={
              chartData.filter((point) => point.dns !== null).length <= 3
                ? { r: 3, strokeWidth: 0 }
                : false
            }
            activeDot={{ r: 5, stroke: "#111820", strokeWidth: 2 }}
            connectNulls={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
