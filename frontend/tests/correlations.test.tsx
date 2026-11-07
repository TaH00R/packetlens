import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { MonitoringProvider } from "../context/MonitoringContext";
import CorrelationFeed from "../components/analytics/CorrelationFeed";
import TimeRangeSelector from "../components/dashboard/TimeRangeSelector";
import { parseCorrelations } from "../lib/correlations";
import { correlationEvent, correlationReport } from "./fixtures/correlations";
import { incidentReport } from "./fixtures/incidents";
import { anomalyReport } from "./fixtures/anomalies";
import { availability, report } from "./fixtures/analytics";
after(() => dom.window.close());

test("correlation validation rejects invalid members, policies and lifecycle", () => {
  assert.equal(parseCorrelations(correlationReport()).count, 0);
  assert.equal(parseCorrelations(correlationReport("all", [correlationEvent(true)])).count, 1);
  for (const change of [
    (e: ReturnType<typeof correlationEvent>) => { e.members[1] = e.members[0] },
    (e: ReturnType<typeof correlationEvent>) => { e.members[1].target = e.members[0].target },
    (e: ReturnType<typeof correlationEvent>) => { e.policy.min_targets = 1 },
    (e: ReturnType<typeof correlationEvent>) => { e.policy.window_ns = 0 },
    (e: ReturnType<typeof correlationEvent>) => { e.ended_at = "2026-11-07T11:00:00Z" },
  ]) { const event = correlationEvent(); change(event); assert.throws(() => parseCorrelations(correlationReport("15m", [event]))) }
  const duplicate = correlationEvent(); assert.throws(() => parseCorrelations(correlationReport("15m", [duplicate, duplicate])));
});

test("correlation feed renders uncertainty, affected targets, ended overlaps, filters and pagination", async t => {
  const calls: string[] = [];
  t.mock.method(globalThis, "fetch", async (input: RequestInfo | URL) => {
    const u = new URL(String(input)); calls.push(u.toString()); const range = u.searchParams.get("range") ?? "15m";
    if (u.pathname === "/api/correlations") {
      const old = u.searchParams.has("before"); const event = correlationEvent(old); if (old) event.id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-0";
      const body = correlationReport(range, [event]); body.has_more = !old; body.next_cursor = old ? "" : "older-correlations"; return new Response(JSON.stringify(body));
    }
    return new Response(JSON.stringify(u.pathname === "/api/incidents" ? incidentReport(range) : u.pathname === "/api/anomalies" ? anomalyReport(range) : u.pathname === "/api/analytics" ? report() : u.pathname === "/api/availability" ? availability() : { count: 0, snapshots: [] }));
  });
  const container = document.createElement("div"); document.body.appendChild(container); const root = createRoot(container);
  try {
    await act(async () => root.render(<MonitoringProvider><TimeRangeSelector /><CorrelationFeed /></MonitoringProvider>));
    assert.match(container.textContent ?? "", /No confirmed cause/); assert.match(container.textContent ?? "", /Current connectivity is unknown/); assert.match(container.textContent ?? "", /TCP · a:443/); assert.match(container.textContent ?? "", /DNS · example.test/);
    const older = [...container.querySelectorAll("button")].find(b => b.textContent === "Older correlations"); assert.ok(older && !older.disabled); await act(async () => older.click());
    assert.match(container.textContent ?? "", /Overlap ended/); assert.match(container.textContent ?? "", /does not confirm recovery/); assert.ok(calls.some(v => v.includes("before=older-correlations") && v.includes("to=")));
    const five = [...container.querySelectorAll("button")].find(b => b.textContent === "5m"); assert.ok(five); await act(async () => five.click()); assert.match(container.textContent ?? "", /Page 1/);
    const select = container.querySelector<HTMLSelectElement>('select[aria-label="Correlation probe type"]'); assert.ok(select); await act(async () => { select.value = "dns"; select.dispatchEvent(new Event("change", { bubbles: true })) }); assert.ok(calls.some(v => v.includes("type=dns") && v.includes("range=5m")));
  } finally { await act(async () => root.unmount()); container.remove() }
});

test("correlation errors and empty reports do not imply healthy connectivity", async t => {
  let fail = false;
  t.mock.method(console, "error", () => {});
  t.mock.method(globalThis, "fetch", async (input: RequestInfo | URL) => {
    const u = new URL(String(input)); if (fail) return new Response("unavailable", { status: 503 });
    return new Response(JSON.stringify(u.pathname === "/api/correlations" ? correlationReport() : u.pathname === "/api/incidents" ? incidentReport() : u.pathname === "/api/anomalies" ? anomalyReport() : u.pathname === "/api/analytics" ? report() : u.pathname === "/api/availability" ? availability() : { count: 0, snapshots: [] }));
  });
  const container = document.createElement("div"); const root = createRoot(container);
  try {
    await act(async () => root.render(<MonitoringProvider><CorrelationFeed /></MonitoringProvider>)); assert.match(container.textContent ?? "", /Missing measurements do not establish healthy connectivity/);
    await act(async () => root.unmount()); fail = true; const next = createRoot(container);
    try { await act(async () => next.render(<MonitoringProvider><CorrelationFeed /></MonitoringProvider>)); assert.ok(container.querySelector('[role="alert"]')); assert.doesNotMatch(container.textContent ?? "", /0 correlation events/) } finally { await act(async () => next.unmount()) }
  } finally { container.remove() }
});
