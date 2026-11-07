import PageFrame from "@/components/layout/PageFrame";
import PersistedOverview from "@/components/analytics/PersistedOverview";
export default function Page() {
  return (
    <PageFrame
      title="Overview"
      description="Successful latency and observed connectivity across your configured targets."
    >
      <PersistedOverview />
    </PageFrame>
  );
}
