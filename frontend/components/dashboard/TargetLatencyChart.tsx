
"use client";

import { useMemo, useState } from "react";
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
  MonitoringSnapshot,
  ProbeResult,
} from "@/types/metrics";

interface Props {
  results: MonitoringSnapshot[];
}

const COLORS = [
  "#14B8A6",
  "#3B82F6",
  "#A78BFA",
  "#F59E0B",
  "#EC4899",
  "#22C55E",
];

type ProbeType = "tcp" | "dns";

export default function TargetLatencyChart({ results }: Props) {
  const [probeType, setProbeType] = useState<ProbeType>("tcp");

  const recent = results.slice(-60);

  const targets = useMemo(() => {
    const unique = new Set<string>();

    recent.forEach((snapshot) => {
      const probes =
        probeType === "tcp" ? snapshot.tcp : snapshot.dns;

      probes.forEach((probe) => unique.add(probe.target));
    });

    return Array.from(unique);
  }, [recent, probeType]);

  const chartData = useMemo(() => {
    return recent.map((snapshot) => {
      const probes: ProbeResult[] =
        probeType === "tcp" ? snapshot.tcp : snapshot.dns;

      const point: Record<string, string | number | null> = {
        timestamp: new Date(snapshot.timestamp).getTime(),
      };

      targets.forEach((target) => {
        const result = probes.find((probe) => probe.target === target);

        point[target] =
          result && result.success ? result.latency_ms : null;
      });

      return point;
    });
  }, [recent, targets, probeType]);

  return (
    <section className="rounded-xl border border-white/[0.07] bg-[#111820]">
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-white/[0.07] p-5">
        <div>
          <h3 className="font-medium text-white">
            Target latency
          </h3>
          <p className="mt-1 text-xs text-gray-500">
            Compare latency across individual destinations
          </p>
        </div>

        <div className="flex rounded-lg border border-white/[0.08] bg-[#0B1016] p-1">
          {(["tcp", "dns"] as const).map((type) => (
            <button
              key={type}
              onClick={() => setProbeType(type)}
              className={`rounded-md px-4 py-2 text-xs font-medium uppercase transition-colors ${
                probeType === type
                  ? "bg-teal-500/15 text-teal-400"
                  : "text-gray-500 hover:text-white"
              }`}
            >
              {type}
            </button>
          ))}
        </div>
      </div>

      <div className="p-5">
        {targets.length === 0 ? (
          <div className="flex h-[300px] items-center justify-center text-sm text-gray-500">
            Waiting for measurements...
          </div>
        ) : (
          <>
            <div className="mb-5 flex flex-wrap gap-4">
              {targets.map((target, index) => (
                <div
                  key={target}
                  className="flex items-center gap-2 text-xs text-gray-400"
                >
                  <span
                    className="h-2 w-2 rounded-full"
                    style={{
                      backgroundColor: COLORS[index % COLORS.length],
                    }}
                  />
                  {target}
                </div>
              ))}
            </div>

            <div className="h-[300px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart
                  data={chartData}
                  margin={{
                    top: 10,
                    right: 20,
                    left: 0,
                    bottom: 0,
                  }}
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
                    labelFormatter={(value) =>
                      new Date(Number(value)).toLocaleTimeString()
                    }
                    formatter={(value, name) => [
                      value == null ? "No measurement" : `${value} ms`,
                      String(name),
                    ]}
                  />

                  {targets.map((target, index) => (
                    <Line
                      key={target}
                      type="monotone"
                      dataKey={target}
                      name={target}
                      stroke={COLORS[index % COLORS.length]}
                      strokeWidth={2}
                      dot={false}
                      activeDot={{ r: 4 }}
                      connectNulls={false}
                      isAnimationActive
                      animationDuration={700}
                      animationEasing="ease-in-out"
                    />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
