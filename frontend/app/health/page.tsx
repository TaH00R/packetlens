import PageFrame from "@/components/layout/PageFrame";
import AvailabilityPanel from "@/components/analytics/AvailabilityPanel";
export default function Page() {
  return (
    <PageFrame
      title="Availability"
      description="Observed successes, outcome streaks, and the last known state of each target."
    >
      <AvailabilityPanel />
    </PageFrame>
  );
}
