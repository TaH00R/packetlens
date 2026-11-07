import PageFrame from "@/components/layout/PageFrame";
import LatencyAnalytics from "@/components/analytics/LatencyAnalytics";
export default function Page() {
  return (
    <PageFrame
      title="TCP analytics"
      description="Connection timing across retained TCP measurements."
    >
      <LatencyAnalytics type="tcp" />
    </PageFrame>
  );
}
