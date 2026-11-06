import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MonitoringProvider } from "../context/MonitoringContext";
import LatencyAnalytics from "../components/analytics/LatencyAnalytics";
import AvailabilityPanel from "../components/analytics/AvailabilityPanel";
import TimeRangeSelector from "../components/dashboard/TimeRangeSelector";
import { incidentReport } from "./fixtures/incidents";
import { anomalyReport } from "./fixtures/anomalies";
import { availability, report } from "./fixtures/analytics";
import type { TimeRange } from "../lib/monitoring";

after(() => dom.window.close());

function fixture(root: Root) {
  root.render(<MonitoringProvider><TimeRangeSelector /><LatencyAnalytics type="tcp" /><AvailabilityPanel type="tcp" /></MonitoringProvider>);
}
function card(container: HTMLElement, title: string) {
  const label = [...container.querySelectorAll("span")].find((n) => n.textContent === title);
  return label?.closest("div.rounded-xl")?.querySelector("h2")?.textContent;
}

test("renders percentiles and availability, and target/window selectors respond", async (t) => {
  const signals: AbortSignal[] = [];
  const fetcher: typeof fetch = async (input, options) => {
    if (options?.signal) signals.push(options.signal);
    const url = new URL(String(input));
    const body = url.pathname === "/api/incidents" ? incidentReport(url.searchParams.get("range") ?? "15m") : url.pathname === "/api/anomalies" ? anomalyReport(url.searchParams.get("range") ?? "15m") : url.pathname === "/api/analytics" ? report(url.searchParams.get("range") as TimeRange) : url.pathname === "/api/availability" ? availability() : { count:0, snapshots:[] };
    return new Response(JSON.stringify(body), { status:200 });
  };
  t.mock.method(globalThis,"fetch",fetcher);
  const container = document.createElement("div"); document.body.appendChild(container);
  const root = createRoot(container);
  try {
    await act(async () => fixture(root));
    assert.equal(card(container,"Average latency"),"50.00 ms");
    assert.equal(card(container,"P95 latency"),"90.00 ms");
    assert.equal(card(container,"TCP availability · 1h"),"100.00%");
    const select = container.querySelector<HTMLSelectElement>('select[aria-label="TCP analytics target"]');
    assert.ok(select);
    await act(async () => { select.value="alpha:443"; select.dispatchEvent(new Event("change",{ bubbles:true })); });
    assert.equal(card(container,"Average latency"),"10.00 ms");
    const windowSelect = [...container.querySelectorAll("select")].find((s) => [...s.options].some((o) => o.value === "7d"));
    assert.ok(windowSelect);
    await act(async () => { windowSelect.value="7d"; windowSelect.dispatchEvent(new Event("change",{ bubbles:true })); });
    assert.equal(windowSelect.value,"7d");
    assert.match(container.textContent ?? "", /missing measurements are unknown/i);
  } finally {
    await act(async () => root.unmount()); container.remove();
  }
  assert.ok(signals.every((signal) => signal.aborted),"unmount must abort outstanding report/history controllers");
});

test("empty charts and unavailable datasets do not show invented zero latencies", async (t) => {
  t.mock.method(globalThis,"fetch",async (input: RequestInfo | URL) => {
    const path = new URL(String(input)).pathname;
    return new Response(JSON.stringify(path === "/api/incidents" ? incidentReport() : path === "/api/anomalies" ? anomalyReport() : path === "/api/analytics" ? report("15m",50,true) : path === "/api/availability" ? availability(true) : { count:0,snapshots:[] }));
  });
  const container=document.createElement("div"); document.body.appendChild(container); const root=createRoot(container);
  try {
    await act(async () => fixture(root));
    assert.equal(card(container,"P99 latency"),"No data");
    assert.equal(card(container,"TCP availability · 1h"),"No observations");
    assert.match(container.textContent ?? "",/No successful latency measurements/);
    assert.match(container.textContent ?? "",/No targets observed/);
  } finally { await act(async () => root.unmount()); container.remove(); }
});

test("late responses for an old range cannot overwrite the new selection", async (t) => {
  let finishOld: ((response: Response) => void) | undefined;
  t.mock.method(globalThis,"fetch",async (input: RequestInfo | URL) => {
    const url = new URL(String(input));
    if (url.pathname === "/api/analytics" && url.searchParams.get("range") === "15m") return await new Promise<Response>((resolve) => { finishOld=resolve });
    return new Response(JSON.stringify(url.pathname === "/api/incidents" ? incidentReport(url.searchParams.get("range") ?? "15m") : url.pathname === "/api/anomalies" ? anomalyReport(url.searchParams.get("range") ?? "15m") : url.pathname === "/api/analytics" ? report("1h",100) : url.pathname === "/api/availability" ? availability() : { count:0,snapshots:[] }));
  });
  const container=document.createElement("div"); document.body.appendChild(container); const root=createRoot(container);
  try {
    await act(async () => fixture(root));
    const hour = [...container.querySelectorAll("button")].find((b) => b.textContent === "1h"); assert.ok(hour);
    await act(async () => hour.dispatchEvent(new MouseEvent("click",{ bubbles:true })));
    assert.equal(card(container,"Average latency"),"100.00 ms");
    assert.ok(finishOld);
    await act(async () => finishOld?.(new Response(JSON.stringify(report("15m",50)))));
    assert.equal(card(container,"Average latency"),"100.00 ms");
  } finally { await act(async () => root.unmount()); container.remove(); }
});

test("API errors are visible while recent live monitoring stays mounted", async (t) => {
  t.mock.method(console,"error",() => {});
  t.mock.method(globalThis,"fetch",async () => new Response("unavailable",{ status:503 }));
  const container=document.createElement("div"); document.body.appendChild(container); const root=createRoot(container);
  try {
    await act(async () => fixture(root));
    assert.ok(container.querySelector('[role="alert"]'));
    assert.match(container.textContent ?? "",/could not be refreshed/);
    assert.match(container.textContent ?? "",/Recent live activity remains below/);
  } finally { await act(async () => root.unmount()); container.remove(); }
});
