
"use client";

import {
  Activity,
  CheckCircle2,
  AlertTriangle,
  Gauge,
} from "lucide-react";

import MetricCard from "@/components/dashboard/MetricCard";
import { useMonitoring } from "@/context/MonitoringContext";

import type { ProbeResult } from "@/types/metrics";

interface TargetHealth {
  target: string;
  type: string;
  attempts: number;
  successes: number;
  failures: number;
  successRate: number;
  averageLatency: number | null;
  jitter: number | null;
}

function calculateJitter(results: ProbeResult[]): number | null {
  const successful = results
    .filter((result) => result.success)
    .sort(
      (a, b) =>
        new Date(a.timestamp).getTime() -
        new Date(b.timestamp).getTime()
    );

  if (successful.length < 2) return null;

  let totalDifference = 0;

  for (let i = 1; i < successful.length; i++) {
    totalDifference += Math.abs(
      successful[i].latency_ms - successful[i - 1].latency_ms
    );
  }

  return totalDifference / (successful.length - 1);
}

function buildTargetHealth(results: ProbeResult[]): TargetHealth[] {
  const groups = new Map<string, ProbeResult[]>();

  for (const result of results) {
    const key = `${result.type}:${result.target}`;

    const existing = groups.get(key) ?? [];
    existing.push(result);
    groups.set(key, existing);
  }

  return Array.from(groups.entries()).map(([key, measurements]) => {
    const successful = measurements.filter((result) => result.success);
    const failures = measurements.length - successful.length;

    const averageLatency =
      successful.length > 0
        ? successful.reduce(
            (sum, result) => sum + result.latency_ms,
            0
          ) / successful.length
        : null;

    return {
      target: measurements[0].target,
      type: measurements[0].type,
      attempts: measurements.length,
      successes: successful.length,
      failures,
      successRate: (successful.length / measurements.length) * 100,
      averageLatency,
      jitter: calculateJitter(measurements),
    };
  });
}

function formatLatency(value: number | null) {
  return value === null ? "—" : `${value.toFixed(2)} ms`;
}

export default function NetworkHealthPage() {
  const { history } = useMonitoring();

  const results = history.flatMap((snapshot) => [
    ...snapshot.tcp,
    ...snapshot.dns,
  ]);

  const successful = results.filter((result) => result.success);
  const failed = results.length - successful.length;

  const successRate =
    results.length > 0
      ? (successful.length / results.length) * 100
      : null;

  const averageLatency =
    successful.length > 0
      ? successful.reduce(
          (sum, result) => sum + result.latency_ms,
          0
        ) / successful.length
      : null;

  const targetHealth = buildTargetHealth(results);

  const jitterValues = targetHealth
    .map((target) => target.jitter)
    .filter((value): value is number => value !== null);

  const averageJitter =
    jitterValues.length > 0
      ? jitterValues.reduce((sum, value) => sum + value, 0) /
        jitterValues.length
      : null;

  return (
    <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
      <section>
        <div className="mb-3 inline-flex rounded-md border border-purple-500/20 bg-purple-500/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-purple-400">
          Workspace / Health
        </div>

        <h1 className="text-3xl font-semibold">
          Network Health
        </h1>

        <p className="mt-2 text-sm text-gray-500">
          Understand connectivity reliability and latency stability.
        </p>
      </section>

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="Probe success rate"
          value={successRate === null ? "—" : `${successRate.toFixed(1)}%`}
          description={`${successful.length} successful probes`}
          icon={CheckCircle2}
          color="text-emerald-400"
        />

        <MetricCard
          title="Average latency"
          value={formatLatency(averageLatency)}
          description="Successful TCP and DNS measurements"
          icon={Gauge}
          color="text-teal-400"
        />

        <MetricCard
          title="Average jitter"
          value={formatLatency(averageJitter)}
          description="Mean per-target latency variation"
          icon={Activity}
          color="text-blue-400"
        />

        <MetricCard
          title="Failed probes"
          value={String(failed)}
          description="Failed attempts in current session history"
          icon={AlertTriangle}
          color="text-red-400"
        />
      </section>

      <section className="rounded-xl border border-white/[0.07] bg-[#111820]">
        <div className="border-b border-white/[0.07] p-5">
          <h2 className="font-medium text-white">
            Target health
          </h2>

          <p className="mt-1 text-xs text-gray-500">
            Reliability and latency stability by destination
          </p>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-white/[0.07] text-xs text-gray-500">
              <tr>
                <th className="px-5 py-4 font-medium">Target</th>
                <th className="px-5 py-4 font-medium">Type</th>
                <th className="px-5 py-4 font-medium">Success rate</th>
                <th className="px-5 py-4 font-medium">Avg latency</th>
                <th className="px-5 py-4 font-medium">Jitter</th>
                <th className="px-5 py-4 font-medium">Failures</th>
              </tr>
            </thead>

            <tbody className="divide-y divide-white/[0.05]">
              {targetHealth.map((target) => (
                <tr
                  key={`${target.type}:${target.target}`}
                  className="text-gray-300"
                >
                  <td className="whitespace-nowrap px-5 py-4 font-medium">
                    {target.target}
                  </td>

                  <td className="px-5 py-4 uppercase text-gray-500">
                    {target.type}
                  </td>

                  <td className="px-5 py-4">
                    <span
                      className={
                        target.successRate >= 95
                          ? "text-teal-400"
                          : target.successRate >= 75
                            ? "text-yellow-400"
                            : "text-red-400"
                      }
                    >
                      {target.successRate.toFixed(1)}%
                    </span>
                  </td>

                  <td className="whitespace-nowrap px-5 py-4">
                    {formatLatency(target.averageLatency)}
                  </td>

                  <td className="whitespace-nowrap px-5 py-4">
                    {formatLatency(target.jitter)}
                  </td>

                  <td className="px-5 py-4 text-red-400">
                    {target.failures}
                  </td>
                </tr>
              ))}

              {targetHealth.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-5 py-10 text-center text-gray-500"
                  >
                    Waiting for monitoring measurements...
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <div className="rounded-xl border border-yellow-500/10 bg-yellow-500/[0.04] p-5">
        <h3 className="text-sm font-medium text-yellow-400">
          Measurement note
        </h3>

        <p className="mt-2 text-xs leading-6 text-gray-400">
          Jitter is calculated from absolute differences between
          successive successful measurements for each target.
          Failed probes are excluded from the jitter calculation.
          Probe success rate measures the success of our TCP and DNS
          checks; it is not a direct measurement of packet-level loss.
        </p>
      </div>
    </div>
  );
}
