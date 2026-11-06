
import { ShieldCheck } from "lucide-react";
import type { ProbeResult } from "@/types/metrics";

interface Props {
  results: ProbeResult[];
}

export default function NetworkHealth({ results }: Props) {
  const groups = [
    { label: "TCP", type: "tcp" },
    { label: "DNS", type: "dns" },
  ];

  return (
    <section className="rounded-xl border border-white/[0.07] bg-[#111820] p-5">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-medium text-white">Probe health</h3>
          <p className="mt-1 text-xs text-gray-500">
            Current session performance
          </p>
        </div>
        <ShieldCheck className="text-teal-400" size={20} />
      </div>

      <div className="mt-7 space-y-6">
        {groups.map((group) => {
          const samples = results.filter(
            (r) => r.type === group.type
          );

          const successful = samples.filter((r) => r.success).length;

          const rate = samples.length
            ? (successful / samples.length) * 100
            : 0;

          return (
            <div key={group.type}>
              <div className="mb-2 flex justify-between text-xs">
                <span className="text-gray-400">
                  {group.label} probes
                </span>
                <span className="text-gray-200">
                  {rate.toFixed(1)}%
                </span>
              </div>

              <div className="h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                <div
                  className="h-full rounded-full bg-teal-400 transition-all"
                  style={{ width: `${rate}%` }}
                />
              </div>
            </div>
          );
        })}
      </div>

      <div className="mt-7 grid grid-cols-2 gap-3">
        <div className="rounded-lg bg-white/[0.03] p-4">
          <p className="text-xs text-gray-500">Successful</p>
          <p className="mt-2 text-xl font-semibold text-teal-400">
            {results.filter((r) => r.success).length}
          </p>
        </div>

        <div className="rounded-lg bg-white/[0.03] p-4">
          <p className="text-xs text-gray-500">Failed</p>
          <p className="mt-2 text-xl font-semibold text-red-400">
            {results.filter((r) => !r.success).length}
          </p>
        </div>
      </div>
    </section>
  );
}
