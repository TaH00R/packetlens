import PageFrame from "@/components/layout/PageFrame";
import IncidentTimeline from "@/components/analytics/IncidentTimeline";
export default function Page() {
  return (
    <PageFrame
      title="Failure incidents"
      description="Sustained target failures, recovery confirmation, and elapsed episode spans."
    >
      <IncidentTimeline />
    </PageFrame>
  );
}
