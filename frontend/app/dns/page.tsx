import PageFrame from "@/components/layout/PageFrame";
import LatencyAnalytics from "@/components/analytics/LatencyAnalytics";
export default function Page() {
  return (
    <PageFrame
      title="DNS analytics"
      description="System DNS lookup timing across retained measurements."
    >
      <LatencyAnalytics type="dns" />
    </PageFrame>
  );
}
