
import { Globe, Network, Radio } from "lucide-react";
import type { ProbeResult } from "@/types/metrics";

interface Props {
  results: ProbeResult[];
}

export default function ProbeFeed({ results }: Props) {
  const recent = [...results].reverse().slice(0, 10);

  return (
    <section className="rounded-xl border border-white/[0.07] bg-[#111820]">
      <div className="border-b border-white/[0.07] p-5">
        <div className="flex items-center justify-between">
          <h3 className="font-medium text-white">
            Live probe activity
          </h3>
          <Radio size={18} className="text-teal-400" />
        </div>

        <p className="mt-1 text-xs text-gray-500">
          Latest network measurements
        </p>
      </div>

      <div className="divide-y divide-white/[0.05]">
        {recent.length === 0 ? (
          <p className="p-6 text-sm text-gray-500">
            Waiting for measurements...
          </p>
        ) : (
          recent.map((result, index) => (
            <div
              key={`${result.type}-${result.target}-${result.timestamp}-${index}`}
              className="flex items-center justify-between gap-3 p-4"
            >
              <div className="flex min-w-0 items-center gap-3">
                {result.type === "dns" ? (
                  <Globe size={17} className="text-blue-400" />
                ) : (
                  <Network size={17} className="text-teal-400" />
                )}

                <div className="min-w-0">
                  <p className="truncate text-sm text-gray-200">
                    {result.target}
                  </p>
                  <p className="text-[10px] text-gray-600">
                    {new Date(result.timestamp).toLocaleTimeString()}
                  </p>
                </div>
              </div>

              <span
                className={`shrink-0 text-sm ${
                  result.success ? "text-teal-400" : "text-red-400"
                }`}
              >
                {result.success
                  ? `${result.latency_ms.toFixed(2)} ms`
                  : "Failed"}
              </span>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
