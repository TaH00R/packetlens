import type { TimeRange } from "./monitoring";
export type ExportDataset = "measurements" | "dns-resolvers" | "incidents";
export interface ExportSelection { dataset: ExportDataset; range: TimeRange; type: "" | "tcp" | "dns"; target: string; to: string }
export const MAX_EXPORT_BYTES = 32 * 1024 * 1024;
const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
export function exportURL(selection: ExportSelection): string {
  const params = new URLSearchParams({ range: selection.range, to: selection.to });
  if (selection.type) params.set("type", selection.type);
  if (selection.target.trim()) params.set("target", selection.target.trim());
  return `${API_URL}/api/export/${selection.dataset}.csv?${params}`;
}
// Keep browser memory bounded. Larger exports can be streamed straight to a file
// with the generated URL in a terminal, without a JavaScript Blob buffer.
export async function fetchCSV(url: string, signal: AbortSignal, progress: (bytes: number) => void, maxBytes = MAX_EXPORT_BYTES): Promise<Blob> {
  const response = await fetch(url, { signal, cache: "no-store" });
  if (!response.ok) {
    if (response.status === 429) throw new Error("Another export is running or the export rate limit was reached. Wait a moment and retry.");
    throw new Error(`CSV export failed (${response.status}). No file was saved.`);
  }
  if (!response.headers.get("Content-Type")?.toLowerCase().startsWith("text/csv")) { await response.body?.cancel(); throw new Error("The server returned an invalid CSV response. No file was saved.") }
  if (!response.body) throw new Error("The CSV response has no readable body.");
  const reader = response.body.getReader(); const chunks: ArrayBuffer[] = []; let bytes = 0;
  const abort = () => { void reader.cancel().catch(() => {}) };
  signal.addEventListener("abort", abort, { once: true });
  try {
    while (true) {
      if (signal.aborted) throw new DOMException("Export cancelled", "AbortError");
      const { done, value } = await reader.read(); if (done) break;
      bytes += value.byteLength;
      if (bytes > maxBytes) throw new Error("This export exceeds the 32 MiB browser download limit. Narrow the range or target, or use the export URL to stream it to a file.");
      chunks.push(value.slice().buffer); progress(bytes);
    }
    if (signal.aborted) throw new DOMException("Export cancelled", "AbortError");
    return new Blob(chunks, { type: "text/csv;charset=utf-8" });
  } catch (error) {
    try { await reader.cancel() } catch { /* The connection may already be closed. */ }
    throw error;
  } finally { signal.removeEventListener("abort", abort); reader.releaseLock() }
}
export function csvFilename(selection: ExportSelection): string { return `packetlens-${selection.dataset}-${selection.range}-${selection.to.replace(/[:.]/g, "-")}.csv` }
