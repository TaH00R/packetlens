import test from "node:test";
import assert from "node:assert/strict";
import { parseIncidents, formatIncidentDuration } from "../lib/incidents.ts";
import { incidentReport, failureIncident } from "./fixtures/incidents.ts";

test("incident responses distinguish empty, open, and recovered records",()=>{
  assert.equal(parseIncidents(incidentReport()).count,0);
  assert.equal(parseIncidents(incidentReport("15m",[failureIncident()])).incidents[0].duration_ms,null);
  assert.equal(parseIncidents(incidentReport("all",[failureIncident(true)],"recovered")).incidents[0].duration_ms,120000);
});
test("invalid lifecycle timestamps, policies, counts, and statuses are rejected",()=>{
  for(const change of [
    v=>{v.count=5}, v=>{v.open_count=-1}, v=>{v.has_more=true}, v=>{v.status="confirmed root cause"},
    v=>{v.incidents[0].id="invalid"}, v=>{v.incidents[0].policy.failure_threshold=0}, v=>{v.incidents[0].policy.max_gap_ns=Infinity},
    v=>{v.incidents[0].failed_probes=1}, v=>{v.incidents[0].duration_ms=0}, v=>{v.incidents[0].detected_at="invalid"},
    v=>{v.incidents[0].detected_at="2026-11-06T11:00:00Z"}, v=>{v.incidents[0].recovered_at="2026-11-06T12:01:00Z"},
    v=>{v.status="recovered"}, v=>{v.incidents[0].recovery_successes=3},
  ]){const value=incidentReport("15m",[failureIncident()]);change(value);assert.throws(()=>parseIncidents(value),/Invalid incident/)}
  const badDuration=incidentReport("15m",[failureIncident(true)]);badDuration.incidents[0].duration_ms=5;
  assert.throws(()=>parseIncidents(badDuration),/Invalid incident/);
});
test("elapsed duration retains zero values and distinguishes ongoing incidents",()=>{
  assert.equal(formatIncidentDuration(null),"Awaiting recovery");
  assert.equal(formatIncidentDuration(0),"0s");
  assert.equal(formatIncidentDuration(120000),"2m 0s");
  assert.equal(formatIncidentDuration(3600000),"1h 0m");
  assert.equal(formatIncidentDuration(86400000),"1d 0h");
});
