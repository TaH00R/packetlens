"use client";

import { chartGrid, chartTime, chartTooltipStyle } from "@/lib/chart-style";
import { useMemo, useState } from "react";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  ReferenceDot,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { MonitoringSnapshot, ProbeResult } from "@/types/metrics";
import type { LatencyAnomaly } from "@/types/anomalies";

type ProbeType = "tcp" | "dns";

interface Props {
  anomalies?: LatencyAnomaly[];
  results: MonitoringSnapshot[];
  initialType?: ProbeType;
  availableTypes?: ProbeType[];
}

const COLORS = [
  "#14B8A6",
  "#3B82F6",
  "#A78BFA",
  "#F59E0B",
  "#EC4899",
  "#22C55E",
];

export default function TargetLatencyChart({
  anomalies,
  results,
  initialType = "tcp",
  availableTypes = ["tcp", "dns"],
}: Props) {
  const [probeType, setProbeType] = useState<ProbeType>(initialType);

  const recent = results;

  const targets = useMemo(() => {
    const unique = new Set<string>();

    recent.forEach((snapshot) => {
      const probes = probeType === "tcp" ? snapshot.tcp : snapshot.dns;

      probes.forEach((probe) => {
        unique.add(probe.target);
      });
    });

    return Array.from(unique);
  }, [recent, probeType]);

  const markers = (anomalies ?? []).filter(
    (e) =>
      e.type === probeType &&
      targets.includes(e.target) &&
      recent.length > 0 &&
      Date.parse(e.cycle_timestamp) >= Date.parse(recent[0].timestamp) &&
      Date.parse(e.cycle_timestamp) <=
        Date.parse(recent[recent.length - 1].timestamp),
  );

  const chartData = useMemo(() => {
    return recent.map((snapshot) => {
      const probes: ProbeResult[] =
        probeType === "tcp" ? snapshot.tcp : snapshot.dns;

      const point: Record<string, string | number | null> = {
        timestamp: new Date(snapshot.timestamp).getTime(),
      };

      targets.forEach((target) => {
        const result = probes.find((probe) => probe.target === target);

        point[target] = result && result.success ? result.latency_ms : null;
      });

      return point;
    });
  }, [recent, targets, probeType]);

  return (
    <section className="rounded-xl border border-white/[0.07] bg-[#111820]">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-white/[0.07] p-5">
        <div>
          <h3 className="font-medium text-white">Target latency</h3>

          <p className="mt-1 text-xs text-gray-500">
            Compare latency across individual destinations
          </p>
        </div>

        {/* Probe type selector */}
        <div className="flex rounded-lg border border-white/[0.08] bg-[#0B1016] p-1">
          {availableTypes.map((type) => (
            <button
              key={type}
              onClick={() => setProbeType(type)}
              aria-pressed={probeType === type}
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

      {/* Chart content */}
      <div className="p-5">
        {anomalies && (
          <p className="mb-3 text-xs text-red-300">
            Red dots mark onset measurements from the newest loaded anomaly-feed
            page; older onsets may not be shown here.
          </p>
        )}
        {targets.length === 0 ? (
          <div className="flex h-[300px] items-center justify-center text-sm text-gray-500">
            Waiting for measurements...
          </div>
        ) : (
          <>
            {/* Target legend */}
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

            {/* Latency chart */}
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
                  <CartesianGrid {...chartGrid} />

                  <XAxis
                    dataKey="timestamp"
                    type="number"
                    scale="time"
                    domain={["dataMin", "dataMax"]}
                    tickFormatter={chartTime}
                    tick={{
                      fill: "#8492a6",
                      fontSize: 11,
                    }}
                    axisLine={false}
                    tickLine={false}
                    minTickGap={75}
                    tickMargin={10}
                  />

                  <YAxis
                    tick={{
                      fill: "#8492a6",
                      fontSize: 11,
                    }}
                    axisLine={false}
                    tickLine={false}
                    width={65}
                    tickFormatter={(value: number) => `${value} ms`}
                  />

                  <Tooltip
                    contentStyle={chartTooltipStyle}
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
                      dataKey={(point) => point[target]}
                      name={target}
                      stroke={COLORS[index % COLORS.length]}
                      strokeWidth={2.5}
                      strokeLinecap="round"
                      dot={
                        chartData.filter((point) => point[target] !== null)
                          .length <= 3
                          ? { r: 3, strokeWidth: 0 }
                          : false
                      }
                      activeDot={{ r: 5, stroke: "#111820", strokeWidth: 2 }}
                      connectNulls={false}
                      isAnimationActive={false}
                    />
                  ))}
                  {markers.map((e) => (
                    <ReferenceDot
                      key={e.id}
                      x={Date.parse(e.cycle_timestamp)}
                      y={e.observed_latency_ms}
                      r={4}
                      fill="#F87171"
                      stroke="#F87171"
                      ifOverflow="extendDomain"
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
