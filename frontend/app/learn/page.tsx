
import {
  Activity,
  Clock,
  Globe,
  Network,
  ShieldCheck,
} from "lucide-react";

const metrics = [
  {
    title: "Latency",
    icon: Clock,
    color: "text-teal-400",
    description: "How long a network operation takes.",
    explanation:
      "PacketLens measures the time taken to establish a TCP connection or resolve a DNS query. Lower latency generally means a faster response.",
    example:
      "A TCP connection measurement of 40 ms means the connection was established in approximately 40 milliseconds.",
  },
  {
    title: "TCP connection time",
    icon: Network,
    color: "text-blue-400",
    description: "Time required to establish a TCP connection.",
    explanation:
      "PacketLens attempts to connect to a configured TCP destination and records how long the connection attempt takes. This is not the same as ICMP ping or round-trip time (RTT).",
    example:
      "If connecting to 1.1.1.1:443 takes 55 ms, the observed TCP connection establishment time was 55 ms.",
  },
  {
    title: "DNS resolution time",
    icon: Globe,
    color: "text-purple-400",
    description: "Time required to resolve a domain name.",
    explanation:
      "DNS translates domain names into IP addresses. PacketLens records the duration of its DNS lookup operation. Resolver caching can make repeated lookups considerably faster.",
    example:
      "A lookup of google.com taking 12 ms means the DNS operation completed in approximately 12 milliseconds.",
  },
  {
    title: "Jitter",
    icon: Activity,
    color: "text-orange-400",
    description: "Variation in latency over time.",
    explanation:
      "PacketLens calculates the average absolute difference between successive successful latency measurements for each target. Higher values indicate less consistent latency.",
    example:
      "Measurements of 20 ms, 25 ms and 40 ms produce absolute differences of 5 ms and 15 ms. The average jitter is 10 ms.",
  },
  {
    title: "Probe success rate",
    icon: ShieldCheck,
    color: "text-emerald-400",
    description: "Percentage of successful monitoring attempts.",
    explanation:
      "This measures how often PacketLens successfully completes its configured TCP and DNS probes. It is an application-level probe success rate, not packet-level delivery statistics.",
    example:
      "If 90 out of 100 probes succeed, the probe success rate is 90%.",
  },
];

export default function LearnPage() {
  return (
    <div className="mx-auto max-w-[1200px] space-y-10 p-5 sm:p-8">
      <section>
        <div className="mb-3 inline-flex rounded-md border border-teal-500/20 bg-teal-500/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-teal-400">
          Resources / Documentation
        </div>

        <h1 className="text-3xl font-semibold tracking-tight">
          Understanding Network Metrics
        </h1>

        <p className="mt-3 max-w-2xl text-sm leading-7 text-gray-400">
          Learn what PacketLens measures, how its metrics are
          calculated, and how to interpret the information
          presented in your dashboard.
        </p>
      </section>

      <section className="space-y-5">
        {metrics.map((metric, index) => {
          const Icon = metric.icon;

          return (
            <article
              key={metric.title}
              className="rounded-xl border border-white/[0.07] bg-[#111820] p-6"
            >
              <div className="flex items-start gap-4">
                <div className="rounded-lg border border-white/[0.07] bg-[#0B1016] p-3">
                  <Icon size={22} className={metric.color} />
                </div>

                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="text-xs text-gray-600">
                      0{index + 1}
                    </span>

                    <h2 className="text-lg font-medium text-white">
                      {metric.title}
                    </h2>
                  </div>

                  <p className="mt-1 text-sm text-gray-400">
                    {metric.description}
                  </p>

                  <p className="mt-4 text-sm leading-7 text-gray-400">
                    {metric.explanation}
                  </p>

                  <div className="mt-4 rounded-lg border border-white/[0.06] bg-[#0B1016] p-4">
                    <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-600">
                      Example
                    </p>

                    <p className="mt-2 text-sm leading-6 text-gray-300">
                      {metric.example}
                    </p>
                  </div>
                </div>
              </div>
            </article>
          );
        })}
      </section>

      <section className="rounded-xl border border-teal-500/15 bg-teal-500/[0.04] p-6">
        <h2 className="font-medium text-teal-400">
          Reading the PacketLens dashboard
        </h2>

        <div className="mt-4 space-y-3 text-sm leading-7 text-gray-400">
          <p>
            <strong className="text-gray-200">Latency charts:</strong>{" "}
            Show how measured connection and resolution times change
            over successive monitoring cycles.
          </p>

          <p>
            <strong className="text-gray-200">Target comparison:</strong>{" "}
            Helps identify destinations that consistently respond
            more slowly than others.
          </p>

          <p>
            <strong className="text-gray-200">Jitter:</strong>{" "}
            Helps identify inconsistent performance, even when
            average latency appears reasonable.
          </p>

          <p>
            <strong className="text-gray-200">Failed probes:</strong>{" "}
            Indicate that a monitoring operation did not complete
            successfully. They do not necessarily mean the entire
            network is disconnected.
          </p>
        </div>
      </section>

      <footer className="border-t border-white/[0.07] pt-5 text-xs text-gray-600">
        PacketLens · Network Observability · Documentation
      </footer>
    </div>
  );
}
