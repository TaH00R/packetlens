import PageFrame from "@/components/layout/PageFrame";
import AnomalyFeed from "@/components/analytics/AnomalyFeed";
export default function Page() {
  return (
    <PageFrame
      title="Latency anomalies"
      description="High-latency episodes, their baselines, and observed recovery."
    >
      <AnomalyFeed />
    </PageFrame>
  );
}
