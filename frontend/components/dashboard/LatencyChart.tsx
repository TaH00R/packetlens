
"use client";

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

  const total = successful.reduce(
    (sum, result) => sum + result.latency_ms,
    0
  );

  return Number((total / successful.length).toFixed(2));
}

export default function LatencyChart({
  results,
}: LatencyChartProps) {
  const chartData = useMemo<LatencyPoint[]>(() => {
    return results.map((snapshot) => ({
      timestamp: new Date(snapshot.timestamp).getTime(),
      tcp: calculateAverage(snapshot.tcp),
      dns: calculateAverage(snapshot.dns),
    }));
  }, [results]);

  if (chartData.length === 0) {
    return <p className="flex h-[320px] items-center justify-center text-sm text-gray-400">No measurements in the selected time range.</p>;
  }

  return (
    <div className="h-[320px] w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={chartData}
          margin={{ top: 10, right: 20, left: 0, bottom: 0 }}
        >
          <CartesianGrid
            stroke="#26313d"
            strokeDasharray="3 3"
            vertical={false}
          />

          <XAxis
            dataKey="timestamp"
            type="number"
            scale="time"
            domain={["dataMin", "dataMax"]}
            tickFormatter={(value) =>
              new Date(value).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
              })
            }
            tick={{ fill: "#8492a6", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            minTickGap={35}
          />

          <YAxis
            tick={{ fill: "#8492a6", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            width={45}
            tickFormatter={(value) => `${value} ms`}
          />

          <Tooltip
            contentStyle={{
              backgroundColor: "#111820",
              border: "1px solid #26313d",
              borderRadius: "10px",
              color: "#fff",
            }}
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
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
            connectNulls={false}
            isAnimationActive={false}
          />

          <Line
            type="monotone"
            dataKey="dns"
            name="dns"
            stroke="#3B82F6"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
            connectNulls={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
