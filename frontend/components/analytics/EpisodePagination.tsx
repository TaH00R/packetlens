interface Props { hasPrevious: boolean; hasMore: boolean; loading: boolean; latest: () => void; previous: () => void; next: () => void; noun: string }
export default function EpisodePagination({ hasPrevious, hasMore, loading, latest, previous, next, noun }: Props) {
  return <div className="flex flex-wrap gap-3 text-xs">
    <button type="button" disabled={!hasPrevious || loading} onClick={latest} className="rounded-lg border border-white/10 px-3 py-2 disabled:opacity-40">Latest {noun}</button>
    <button type="button" disabled={!hasPrevious || loading} onClick={previous} className="rounded-lg border border-white/10 px-3 py-2 disabled:opacity-40">Previous page</button>
    <button type="button" disabled={!hasMore || loading} onClick={next} className="rounded-lg border border-teal-500/30 px-3 py-2 text-teal-300 disabled:opacity-40">Older {noun}</button>
  </div>;
}
