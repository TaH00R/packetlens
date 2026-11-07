import PageFrame from "@/components/layout/PageFrame";
import JitterHealth from "@/components/analytics/JitterHealth";
export default function Page() {
  return (
    <PageFrame
      title="Jitter"
      description="Latency variation, measured independently for each target in the selected range."
    >
      <JitterHealth />
    </PageFrame>
  );
}
