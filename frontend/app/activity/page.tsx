"use client";
import PageFrame from "@/components/layout/PageFrame";
import DataTabs from "@/components/dashboard/DataTabs";
import ActivityTable from "@/components/dashboard/ActivityTable";
import TargetLatencyChart from "@/components/dashboard/TargetLatencyChart";
import { useMonitoring } from "@/context/MonitoringContext";
export default function ActivityPage() {
  const { history, anomalies } = useMonitoring();
  return (
    <PageFrame
      title="Probe activity"
      description="Individual outcomes from recent cached cycles. Use exports for complete retained history."
    >
      <DataTabs
        label="Probe activity views"
        tabs={[
          {
            id: "system",
            label: "TCP & system DNS",
            content: (
              <ActivityTable
                results={history.flatMap((s) => [...s.tcp, ...s.dns])}
              />
            ),
          },
          {
            id: "resolvers",
            label: "Resolver queries",
            content: (
              <ActivityTable
                results={history.flatMap((s) => s.dns_comparisons ?? [])}
              />
            ),
          },
          {
            id: "trends",
            label: "Target trends",
            content: (
              <div className="space-y-4">
                <p className="text-xs text-slate-400">
                  Up to 1,000 cached cycles; failed and missing observations
                  leave gaps.
                </p>
                <TargetLatencyChart
                  results={history}
                  anomalies={anomalies.data?.anomalies}
                />
              </div>
            ),
          },
        ]}
      />
    </PageFrame>
  );
}
