"use client";

import { useState } from "react";
import { useRemoteReport } from "./useRemoteReport";
import type { RemoteReport } from "@/types/analytics";

interface Cursor { before: string; to: string }
interface Page { as_of: string; has_more: boolean; next_cursor: string }

// Keep filtered/older pages bounded and abort them when the range or filters
// change. The newest unfiltered page remains owned by shared monitoring state.
export function useEpisodeFeed<T extends Page>(endpoint: string, parse: (value: unknown) => T, initial: RemoteReport<T>, range: string, filters: Record<string,string>) {
  const filterParameters = new URLSearchParams(Object.entries(filters).filter(([,value]) => value !== ""));
  const key = `${range}:${filterParameters}`;
  const [pagination, setPagination] = useState<{ key: string; pages: Cursor[] }>({ key: "", pages: [] });
  const pages = pagination.key === key ? pagination.pages : [];
  const cursor = pages.at(-1);
  const parameters = new URLSearchParams({ range, limit: "50" });
  filterParameters.forEach((value,key) => parameters.set(key,value));
  if (cursor) { parameters.set("before",cursor.before); parameters.set("to",cursor.to) }
  const remote = useRemoteReport(filterParameters.size > 0 || cursor ? `${endpoint}?${parameters}` : null,parse,15_000,0);
  const state = filterParameters.size > 0 || cursor ? remote : initial;
  return {
    state, pageNumber: pages.length+1, hasPrevious: pages.length>0, isLatest: cursor === undefined,
    latest: () => setPagination({ key, pages: [] }),
    previous: () => setPagination({ key, pages: pages.slice(0,-1) }),
    next: () => { if (state.data?.has_more && !state.loading) setPagination({ key, pages:[...pages,{ before:state.data.next_cursor,to:cursor?.to ?? state.data.as_of }] }) },
  };
}
