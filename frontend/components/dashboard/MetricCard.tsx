
import type { LucideIcon } from "lucide-react";

interface MetricCardProps {
  title: string;
  value: string;
  description: string;
  icon: LucideIcon;
  color: string;
}

export default function MetricCard({
  title,
  value,
  description,
  icon: Icon,
  color,
}: MetricCardProps) {
  return (
    <div className="rounded-xl border border-white/[0.07] bg-[#111820] p-5 transition hover:border-teal-500/30">
      <div className="flex items-center justify-between">
        <span className="text-sm text-gray-400">{title}</span>
        <div className={`rounded-lg bg-white/[0.04] p-2 ${color}`}>
          <Icon size={18} />
        </div>
      </div>

      <div className="mt-5">
        <h2 className="text-3xl font-semibold tracking-tight text-white">
          {value}
        </h2>
        <p className="mt-2 text-xs text-gray-500">
          {description}
        </p>
      </div>
    </div>
  );
}
