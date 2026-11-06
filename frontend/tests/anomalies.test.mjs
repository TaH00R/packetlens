import test from "node:test";
import assert from "node:assert/strict";
import { parseAnomalies } from "../lib/anomalies.ts";
import { anomalyReport, episode } from "./fixtures/anomalies.ts";

test("anomaly responses preserve empty, active, and recovered episodes",()=>{
  assert.equal(parseAnomalies(anomalyReport()).count,0);
  assert.equal(parseAnomalies(anomalyReport("15m",[episode()])).anomalies[0].recovered_at,null);
  assert.ok(parseAnomalies(anomalyReport("all",[episode(true)])).anomalies[0].recovered_at);
});
test("rejects invalid anomaly statistics, policies, cursors and recovery shapes",()=>{
  for(const change of [
    v=>{v.count=5},v=>{v.has_more=true},v=>{v.anomalies[0].id="invalid"},v=>{v.anomalies[0].observed_latency_ms=0},
    v=>{v.anomalies[0].peak_latency_ms=NaN},v=>{v.anomalies[0].severity="confirmed outage"},v=>{v.anomalies[0].type="icmp"},
    v=>{v.anomalies[0].policy.mad_multiplier=Infinity},v=>{v.anomalies[0].policy.min_samples=1000},v=>{v.anomalies[0].baseline_samples=0},
    v=>{v.anomalies[0].recovered_at="2026-11-06T12:01:00Z"},v=>{v.anomalies[0].timestamp="invalid"},
  ]){const value=anomalyReport("15m",[episode()]);change(value);assert.throws(()=>parseAnomalies(value),/Invalid anomaly/)}
});
