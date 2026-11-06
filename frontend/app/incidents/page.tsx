import IncidentTimeline from "@/components/analytics/IncidentTimeline";

export default function IncidentsPage() {
  return <div className="mx-auto max-w-[1550px] space-y-7 p-5 sm:p-8">
    <header><h1 className="text-3xl font-semibold">Failure incidents</h1><p className="mt-2 text-sm text-gray-400">Sustained failures, successful recovery confirmation, and retained incident history.</p></header>
    <IncidentTimeline />
  </div>;
}
