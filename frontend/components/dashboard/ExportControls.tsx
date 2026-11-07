"use client";
import { useEffect, useRef, useState } from "react";
import { Download } from "lucide-react";
import { useMonitoring } from "@/context/MonitoringContext";
import {
  csvFilename,
  exportURL,
  fetchCSV,
  type ExportDataset,
  type ExportSelection,
} from "@/lib/export";
const inputClass =
  "max-w-full rounded-lg border border-white/10 bg-[#111820] px-3 py-2 text-xs text-gray-200";
export default function ExportControls() {
  const { range, analytics, resolverReport } = useMonitoring();
  const [dataset, setDataset] = useState<ExportDataset>("measurements");
  const [type, setType] = useState<"" | "tcp" | "dns">("");
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);
  const [bytes, setBytes] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [lastURL, setLastURL] = useState<string | null>(null);
  const controller = useRef<AbortController | null>(null);
  const objectURLs = useRef(new Map<string, number>());
  useEffect(() => {
    const urls = objectURLs.current;
    return () => {
      controller.current?.abort();
      controller.current = null;
      setBusy(false);
      setError(null);
      setMessage(null);
      for (const [url, timer] of urls) {
        window.clearTimeout(timer);
        URL.revokeObjectURL(url);
      }
      urls.clear();
    };
  }, []);
  const domains =
    dataset === "dns-resolvers"
      ? (resolverReport.data?.targets.map((t) => t.target) ?? [])
      : (analytics.data?.current.targets
          .filter((t) => !type || t.type === type)
          .map((t) => t.target) ?? []);
  async function download() {
    if (controller.current) return;
    const selection: ExportSelection = {
      dataset,
      range,
      type: dataset === "dns-resolvers" ? "dns" : type,
      target,
      to: new Date().toISOString(),
    };
    const url = exportURL(selection);
    const current = new AbortController();
    controller.current = current;
    setBusy(true);
    setBytes(0);
    setError(null);
    setMessage(null);
    setLastURL(url);
    const timeout = window.setTimeout(() => current.abort(), 70_000);
    try {
      const blob = await fetchCSV(url, current.signal, (count) => {
        if (controller.current === current) setBytes(count);
      });
      if (current.signal.aborted || controller.current !== current) return;
      const objectURL = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = objectURL;
      anchor.download = csvFilename(selection);
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      const timer = window.setTimeout(() => {
        URL.revokeObjectURL(objectURL);
        objectURLs.current.delete(objectURL);
      }, 30_000);
      objectURLs.current.set(objectURL, timer);
      setMessage(
        `CSV download started for ${selection.dataset}, range ${selection.range}, ${selection.type || "all probe types"}${selection.target.trim() ? `, target ${selection.target.trim()}` : ""}.`,
      );
    } catch (cause) {
      if (controller.current === current)
        setError(
          current.signal.aborted
            ? "Export cancelled or timed out. No file was saved."
            : cause instanceof Error
              ? cause.message
              : "CSV download was interrupted. No file was saved.",
        );
    } finally {
      window.clearTimeout(timeout);
      if (controller.current === current) {
        controller.current = null;
        setBusy(false);
      }
    }
  }
  return (
    <section className="text-xs text-gray-300">
      <h2 className="text-lg font-medium">
        <Download className="mr-2 inline text-teal-300" size={18} />
        Export CSV
      </h2>
      <div className="mt-3 space-y-3 rounded-xl border border-white/10 bg-[#0D131A] p-4">
        <p className="max-w-3xl leading-6 text-gray-400">
          Export retained data for the global time range ({range}). Measurements
          include system DNS; resolver comparisons and incident history have
          separate files. Filters use an exact target match. Empty selections
          produce a header-only file.
        </p>
        <div className="flex flex-wrap items-end gap-3">
          <label className="space-y-1">
            <span className="block">Dataset</span>
            <select
              aria-label="Export dataset"
              className={inputClass}
              value={dataset}
              disabled={busy}
              onChange={(e) => {
                setDataset(e.target.value as ExportDataset);
                setTarget("");
              }}
            >
              <option value="measurements">Measurements</option>
              <option value="dns-resolvers">DNS resolver comparisons</option>
              <option value="incidents">Incident history</option>
            </select>
          </label>
          <label className="space-y-1">
            <span className="block">Probe type</span>
            <select
              aria-label="Export probe type"
              className={inputClass}
              value={dataset === "dns-resolvers" ? "dns" : type}
              disabled={busy || dataset === "dns-resolvers"}
              onChange={(e) => {
                setType(e.target.value as typeof type);
                setTarget("");
              }}
            >
              <option value="">TCP and DNS</option>
              <option value="tcp">TCP</option>
              <option value="dns">DNS</option>
            </select>
          </label>
          <label className="min-w-0 space-y-1">
            <span className="block">Target (optional)</span>
            <input
              aria-label="Export target"
              list="export-targets"
              className={`${inputClass} w-64`}
              value={target}
              maxLength={512}
              disabled={busy}
              onChange={(e) => setTarget(e.target.value)}
              placeholder="Exact target or leave blank"
            />
            <datalist id="export-targets">
              {[...new Set(domains)].sort().map((domain) => (
                <option key={domain} value={domain} />
              ))}
            </datalist>
          </label>
          <button
            type="button"
            className="rounded-lg border border-teal-500/30 px-3 py-2 text-teal-300 disabled:opacity-40"
            disabled={busy}
            onClick={() => void download()}
          >
            Download CSV
          </button>
          {busy && (
            <button
              type="button"
              className={inputClass}
              onClick={() => controller.current?.abort()}
            >
              Cancel export
            </button>
          )}
        </div>
        {busy && (
          <p role="status">
            Downloading {(bytes / 1024).toFixed(1)} KiB. The range and filters
            were captured when you started.
          </p>
        )}
        {error && (
          <p role="alert" className="text-amber-300">
            {error}
          </p>
        )}
        {message && (
          <p role="status" className="text-teal-300">
            {message}
          </p>
        )}
        {lastURL && (
          <details className="text-gray-400">
            <summary className="cursor-pointer">
              Export URL for large files
            </summary>
            <p className="mt-2 leading-6">
              Browser downloads are limited to 32 MiB. Stream larger exports to
              a file with a tool such as curl using this URL. The server limits
              each export to 60 seconds; split the time window if needed.
            </p>
            <code className="mt-2 block break-all select-all">{lastURL}</code>
          </details>
        )}
      </div>
    </section>
  );
}
