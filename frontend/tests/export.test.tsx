import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { exportURL, fetchCSV, type ExportSelection } from "../lib/export";
import ExportControls from "../components/dashboard/ExportControls";
import TimeRangeSelector from "../components/dashboard/TimeRangeSelector";
import { MonitoringProvider } from "../context/MonitoringContext";
import { resolverReport } from "./fixtures/resolvers";
import { incidentReport } from "./fixtures/incidents";
import { correlationReport } from "./fixtures/correlations";
import { anomalyReport } from "./fixtures/anomalies";
import { availability, report } from "./fixtures/analytics";
after(() => dom.window.close());
const selection: ExportSelection = { dataset: "measurements", range: "15m", type: "dns", target: 'example.test,"quoted"', to: "2026-11-07T12:00:00Z" };
const csv = "timestamp,target\n2026-11-07T12:00:00Z,example.test\n";
const csvResponse = () => new Response(csv, { headers: { "Content-Type": "text/csv; charset=utf-8" } });
function button(container: HTMLElement, text: string) { const value = [...container.querySelectorAll("button")].find(b => b.textContent === text); assert.ok(value); return value }

test("export URLs capture filters and successful CSV bodies download incrementally", async t => {
  const url = new URL(exportURL(selection)); assert.equal(url.pathname, "/api/export/measurements.csv"); assert.equal(url.searchParams.get("target"), selection.target); assert.equal(url.searchParams.get("to"), selection.to);
  const controller = new AbortController(); const progress: number[] = [];
  t.mock.method(globalThis, "fetch", async (_url: RequestInfo | URL, options?: RequestInit) => { assert.equal(options?.signal, controller.signal); assert.equal(options?.cache, "no-store"); return csvResponse() });
  const blob = await fetchCSV(url.toString(), controller.signal, bytes => progress.push(bytes)); assert.equal(await blob.text(), csv); assert.equal(progress.at(-1), Buffer.byteLength(csv));
});
test("download limits, invalid responses and interrupted streams reject partial files", async t => {
  let cancelled = false;
  t.mock.method(globalThis, "fetch", async () => new Response(new ReadableStream({ start(c) { c.enqueue(new Uint8Array(16)) }, cancel() { cancelled = true } }), { headers: { "Content-Type": "text/csv" } }));
  await assert.rejects(fetchCSV("http://localhost/test", new AbortController().signal, () => {}, 8), /browser download limit/); assert.ok(cancelled);
  t.mock.method(globalThis, "fetch", async () => new Response("{}", { status: 429 })); await assert.rejects(fetchCSV("http://localhost/test", new AbortController().signal, () => {}), /rate limit/);
  t.mock.method(globalThis, "fetch", async () => new Response("{}", { headers: { "Content-Type": "application/json" } })); await assert.rejects(fetchCSV("http://localhost/test", new AbortController().signal, () => {}), /invalid CSV response/);
  let reads = 0;
  t.mock.method(globalThis, "fetch", async () => new Response(new ReadableStream({ pull(c) { if (reads++ === 0) c.enqueue(new TextEncoder().encode("partial,row\n")); else c.error(new Error("connection interrupted")) } }), { headers: { "Content-Type": "text/csv" } }));
  await assert.rejects(fetchCSV("http://localhost/test", new AbortController().signal, () => {}), /connection interrupted/);
});
test("cancelling an in-progress download closes its reader", async t => {
  let cancelled = false;
  t.mock.method(globalThis, "fetch", async () => new Response(new ReadableStream({ cancel() { cancelled = true } }), { headers: { "Content-Type": "text/csv" } }));
  const controller = new AbortController(); const pending = fetchCSV("http://localhost/test", controller.signal, () => {}); await Promise.resolve(); controller.abort(); await assert.rejects(pending, /cancelled/); assert.ok(cancelled);
});

test("dashboard export controls select datasets, filters and global range and initiate one complete download", async t => {
  const urls: string[] = [], files: string[] = [], revoked: string[] = []; let blobs = 0;
  t.mock.method(URL, "createObjectURL", () => `blob:export-${++blobs}`); t.mock.method(URL, "revokeObjectURL", (url: string) => { revoked.push(url) });
  t.mock.method(dom.window.HTMLAnchorElement.prototype, "click", function (this: HTMLAnchorElement) { files.push(this.download) });
  t.mock.method(globalThis, "fetch", async (input: RequestInfo | URL) => { const u = new URL(String(input)); const range = u.searchParams.get("range") ?? "15m"; if (u.pathname.startsWith("/api/export/")) { urls.push(u.toString()); return csvResponse() } return new Response(JSON.stringify(u.pathname === "/api/dns/resolvers" ? resolverReport(range) : u.pathname === "/api/correlations" ? correlationReport(range) : u.pathname === "/api/incidents" ? incidentReport(range) : u.pathname === "/api/anomalies" ? anomalyReport(range) : u.pathname === "/api/analytics" ? report() : u.pathname === "/api/availability" ? availability() : { count: 0, snapshots: [] })) });
  const container = document.createElement("div"); document.body.appendChild(container); const root = createRoot(container);
  try {
    await act(async () => root.render(<MonitoringProvider><TimeRangeSelector /><ExportControls /></MonitoringProvider>));
    const five = button(container, "5m"); await act(async () => five.click());
    const probe = container.querySelector<HTMLSelectElement>('select[aria-label="Export probe type"]'); assert.ok(probe); await act(async () => { probe.value = "dns"; probe.dispatchEvent(new Event("change", { bubbles: true })) });
    const target = container.querySelector<HTMLInputElement>('input[aria-label="Export target"]'); assert.ok(target); await act(async () => { Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, "value")?.set?.call(target, "example.test"); target.dispatchEvent(new Event("input", { bubbles: true })) });
    await act(async () => button(container, "Download CSV").click()); assert.equal(urls.length, 1); const first = new URL(urls[0]); assert.equal(first.searchParams.get("range"), "5m"); assert.equal(first.searchParams.get("type"), "dns"); assert.equal(first.searchParams.get("target"), "example.test"); assert.ok(first.searchParams.get("to")); assert.match(files[0], /^packetlens-measurements-5m-/);
    const dataset = container.querySelector<HTMLSelectElement>('select[aria-label="Export dataset"]'); assert.ok(dataset);
    await act(async () => { dataset.value = "incidents"; dataset.dispatchEvent(new Event("change", { bubbles: true })) }); await act(async () => button(container, "Download CSV").click()); assert.equal(new URL(urls[1]).pathname, "/api/export/incidents.csv");
    await act(async () => { dataset.value = "dns-resolvers"; dataset.dispatchEvent(new Event("change", { bubbles: true })) }); assert.ok(probe.disabled); await act(async () => button(container, "Download CSV").click()); assert.equal(new URL(urls[2]).pathname, "/api/export/dns-resolvers.csv"); assert.match(container.textContent ?? "", /CSV download started/);
  } finally { await act(async () => root.unmount()); container.remove() }
  assert.equal(revoked.length, 3);
});
test("export API errors remain visible and never trigger a file save", async t => {
  let blobs = 0; t.mock.method(URL, "createObjectURL", () => { blobs++; return "blob:unexpected" });
  t.mock.method(console, "error", () => {}); t.mock.method(globalThis, "fetch", async () => new Response("unavailable", { status: 503 }));
  const container = document.createElement("div"); const root = createRoot(container);
  try { await act(async () => root.render(<MonitoringProvider><ExportControls /></MonitoringProvider>)); await act(async () => button(container, "Download CSV").click()); assert.match(container.querySelector('[role="alert"]')?.textContent ?? "", /No file was saved/); assert.equal(blobs, 0); assert.equal(button(container, "Download CSV").disabled, false) } finally { await act(async () => root.unmount()) }
});
