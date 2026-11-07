import PageFrame from "@/components/layout/PageFrame";
import CorrelationFeed from "@/components/analytics/CorrelationFeed";
export default function Page() {
  return (
    <PageFrame
      title="Failure correlations"
      description="Overlapping failures across targets can suggest a shared issue. They do not confirm its cause."
    >
      <CorrelationFeed />
    </PageFrame>
  );
}
