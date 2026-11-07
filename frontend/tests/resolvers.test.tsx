import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { MonitoringProvider } from "../context/MonitoringContext";
import ResolverComparison from "../components/analytics/ResolverComparison";
import TimeRangeSelector from "../components/dashboard/TimeRangeSelector";
import { parseResolvers } from "../lib/resolvers";
import { isSnapshot, mergeHistory } from "../lib/monitoring";
import { resolverReport } from "./fixtures/resolvers";
import { incidentReport } from "./fixtures/incidents";
import { correlationReport } from "./fixtures/correlations";
import { anomalyReport } from "./fixtures/anomalies";
import { availability, report } from "./fixtures/analytics";
after(() => dom.window.close());
test("resolver reports validate statistics, outcomes and missing latency", () => {
  assert.equal(parseResolvers(resolverReport()).groups.length, 0); const valid = resolverReport("all", true); assert.equal(parseResolvers(valid).groups[1].avg_latency_ms, null);
  const bad = structuredClone(valid); bad.groups[1].avg_latency_ms = 0; assert.throws(() => parseResolvers(bad));
  const counts = structuredClone(valid); counts.groups[0].timeouts = 2; assert.throws(() => parseResolvers(counts));
  const trend = structuredClone(valid); trend.targets[0].trend[0].attempts = 1; assert.throws(() => parseResolvers(trend));
});
test("snapshot comparison validation and fallback identity retain different resolvers", () => {
  const p = { type: "dns" as const, target: "example.test", success: true, timestamp: "2026-11-07T12:00:00Z", latency_ms: 20, resolver: "127.0.0.1:53", outcome: "success" as const };
  const a = { timestamp: p.timestamp, tcp: [], dns: [], dns_comparisons: [p] }, b = { ...a, dns_comparisons: [{ ...p, resolver: "127.0.0.2:53" }] };
  assert.ok(isSnapshot(a)); assert.equal(mergeHistory([a], [b]).length, 2); assert.equal(isSnapshot({ ...a, dns_comparisons: [{ ...p, outcome: "timeout" }] }), false);
});
test("resolver comparison displays successful latency and failures and follows domain/range controls", async t => {
  const calls: string[] = [];
  t.mock.method(globalThis, "fetch", async (input: RequestInfo | URL) => { const u = new URL(String(input)); calls.push(u.toString()); const range = u.searchParams.get("range") ?? "15m"; return new Response(JSON.stringify(u.pathname === "/api/dns/resolvers" ? resolverReport(range, true) : u.pathname === "/api/correlations" ? correlationReport(range) : u.pathname === "/api/incidents" ? incidentReport(range) : u.pathname === "/api/anomalies" ? anomalyReport(range) : u.pathname === "/api/analytics" ? report() : u.pathname === "/api/availability" ? availability() : { count: 0, snapshots: [] })) });
  const container = document.createElement("div"); document.body.appendChild(container); const root = createRoot(container);
  try { await act(async () => root.render(<MonitoringProvider><TimeRangeSelector /><ResolverComparison /></MonitoringProvider>)); assert.match(container.textContent ?? "", /20.00 ms/); assert.match(container.textContent ?? "", /No data/); assert.match(container.textContent ?? "", /DNS query timed out/);
    const select = container.querySelector<HTMLSelectElement>('select[aria-label="Resolver comparison domain"]'); assert.ok(select); await act(async () => { select.value = "example.test"; select.dispatchEvent(new Event("change", { bubbles: true })) }); assert.match(container.textContent ?? "", /for example.test/);
    const five = [...container.querySelectorAll("button")].find(b => b.textContent === "5m"); assert.ok(five); await act(async () => five.click()); assert.ok(calls.some(v => v.includes("/api/dns/resolvers?range=5m")));
  } finally { await act(async () => root.unmount()); container.remove() }
});
test("empty resolver comparisons explain configuration and API errors remain visible", async t => {
  t.mock.method(console, "error", () => {}); let fail = false;
  t.mock.method(globalThis, "fetch", async (input: RequestInfo | URL) => { if (fail) return new Response("unavailable", { status: 503 }); const u = new URL(String(input)); return new Response(JSON.stringify(u.pathname === "/api/dns/resolvers" ? resolverReport() : u.pathname === "/api/correlations" ? correlationReport() : u.pathname === "/api/incidents" ? incidentReport() : u.pathname === "/api/anomalies" ? anomalyReport() : u.pathname === "/api/analytics" ? report() : u.pathname === "/api/availability" ? availability() : { count: 0, snapshots: [] })) });
  const container = document.createElement("div"); let root = createRoot(container);
  try { await act(async () => root.render(<MonitoringProvider><ResolverComparison /></MonitoringProvider>)); assert.match(container.textContent ?? "", /Configure PACKETLENS_DNS_RESOLVERS/); assert.equal(container.querySelectorAll("table").length, 0); await act(async () => root.unmount()); fail = true; root = createRoot(container); await act(async () => root.render(<MonitoringProvider><ResolverComparison /></MonitoringProvider>)); assert.ok(container.querySelector('[role="alert"]')); assert.doesNotMatch(container.textContent ?? "", /0 failed queries/) } finally { await act(async () => root.unmount()) }
});
