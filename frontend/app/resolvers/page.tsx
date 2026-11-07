import PageFrame from "@/components/layout/PageFrame";
import ResolverComparison from "@/components/analytics/ResolverComparison";
export default function Page() {
  return (
    <PageFrame
      title="DNS resolvers"
      description="Compare direct A-record queries without mixing them into system DNS measurements."
    >
      <ResolverComparison />
    </PageFrame>
  );
}
