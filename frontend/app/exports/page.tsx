import PageFrame from "@/components/layout/PageFrame";
import ExportControls from "@/components/dashboard/ExportControls";
export default function Page() {
  return (
    <PageFrame
      title="Export data"
      description="Download retained measurements, DNS comparisons, or incident history for the selected range."
    >
      <ExportControls />
    </PageFrame>
  );
}
