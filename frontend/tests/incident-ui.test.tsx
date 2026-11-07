import { correlationReport } from "./fixtures/correlations";
import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after,test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { MonitoringProvider } from "../context/MonitoringContext";
import IncidentTimeline from "../components/analytics/IncidentTimeline";
import TimeRangeSelector from "../components/dashboard/TimeRangeSelector";
import { failureIncident,incidentReport } from "./fixtures/incidents";
import { anomalyReport } from "./fixtures/anomalies";
import { availability,report } from "./fixtures/analytics";
import type { IncidentReport } from "../types/incidents";
after(()=>dom.window.close());

test("incident timeline distinguishes lifecycle states and responds to filters, pages, and ranges",async(t)=>{
 const calls:string[]=[];
 t.mock.method(globalThis,"fetch",async(input:RequestInfo|URL)=>{
  const u=new URL(String(input)); if(u.pathname==="/api/correlations") return new Response(JSON.stringify(correlationReport()));calls.push(u.toString());
  const range=u.searchParams.get("range")??"15m";
  if(u.pathname==="/api/incidents"){
   const status=(u.searchParams.get("status")??"all") as IncidentReport["status"];
   const old=u.searchParams.has("before")||status==="recovered";
   const event=failureIncident(old);
   if(old){event.id="ffffffffffffffffffffffffffffffff-0";event.target="old:443"}
   else if(range==="5m")event.target="new-range:443";
   if(u.searchParams.get("type")==="dns"){event.type="dns";event.target="example.test"}
   const body=incidentReport(range,[event],status);body.open_count=1;body.recovered_count=1;
   if(!old&&status==="all"){body.has_more=true;body.next_cursor="older-incidents"}
   return new Response(JSON.stringify(body));
  }
  return new Response(JSON.stringify(u.pathname==="/api/anomalies"?anomalyReport(range):u.pathname==="/api/analytics"?report():u.pathname==="/api/availability"?availability():{count:0,snapshots:[]}));
 });
 const container=document.createElement("div");document.body.appendChild(container);const root=createRoot(container);
 try{
  await act(async()=>root.render(<MonitoringProvider><TimeRangeSelector/><IncidentTimeline/></MonitoringProvider>));
  assert.match(container.textContent??"",/Ongoing incident/);assert.match(container.textContent??"",/First failed probe/);assert.match(container.textContent??"",/Incident detected/);
  assert.match(container.textContent??"",/Current target state is unknown/);
  const older=[...container.querySelectorAll("button")].find(b=>b.textContent==="Older incidents");assert.ok(older&&!older.disabled);
  await act(async()=>{older.click()});
  assert.match(container.textContent??"",/Recovered incident/);assert.match(container.textContent??"",/Elapsed episode span: 2m 0s/);
  assert.match(container.textContent??"",/does not prove continuous downtime/);
  assert.ok(calls.some(v=>v.includes("before=older-incidents")&&v.includes("to=")));
  const five=[...container.querySelectorAll("button")].find(b=>b.textContent==="5m");assert.ok(five);
  await act(async()=>{five.click()});
  assert.match(container.textContent??"",/new-range:443/);assert.match(container.textContent??"",/Page 1/);
  const status=container.querySelector<HTMLSelectElement>('select[aria-label="Incident status"]');assert.ok(status);
  await act(async()=>{status.value="recovered";status.dispatchEvent(new Event("change",{bubbles:true}))});
  assert.match(container.textContent??"",/Recovered incident/);assert.ok(calls.some(v=>v.includes("status=recovered")));
  const type=container.querySelector<HTMLSelectElement>('select[aria-label="Incident probe type"]');assert.ok(type);
  await act(async()=>{type.value="dns";type.dispatchEvent(new Event("change",{bubbles:true}))});
  assert.match(container.textContent??"",/example.test/);assert.ok(calls.some(v=>v.includes("type=dns")&&v.includes("status=recovered")));
 }finally{await act(async()=>root.unmount());container.remove()}
});

test("empty incident history does not turn isolated failures into incidents",async(t)=>{
 t.mock.method(globalThis,"fetch",async(input:RequestInfo|URL)=>{
  const u=new URL(String(input)); if(u.pathname==="/api/correlations") return new Response(JSON.stringify(correlationReport()));return new Response(JSON.stringify(u.pathname==="/api/incidents"?incidentReport():u.pathname==="/api/anomalies"?anomalyReport():u.pathname==="/api/analytics"?report():u.pathname==="/api/availability"?availability():{count:0,snapshots:[]}));
 });
 const container=document.createElement("div");document.body.appendChild(container);const root=createRoot(container);
 try{
  await act(async()=>root.render(<MonitoringProvider><IncidentTimeline/></MonitoringProvider>));
  assert.match(container.textContent??"",/No recorded incidents match/);assert.match(container.textContent??"",/incomplete streaks/);
  assert.equal(container.querySelector('ol[aria-label="Failure incident timeline"]')?.children.length,0);
 }finally{await act(async()=>root.unmount());container.remove()}
});

test("incident API errors are visible and do not present missing reports as zero incidents",async(t)=>{
 t.mock.method(console,"error",()=>{});t.mock.method(globalThis,"fetch",async()=>new Response("unavailable",{status:503}));
 const container=document.createElement("div");document.body.appendChild(container);const root=createRoot(container);
 try{
  await act(async()=>root.render(<MonitoringProvider><IncidentTimeline/></MonitoringProvider>));
  assert.ok(container.querySelector('[role="alert"]'));assert.match(container.textContent??"",/No data/);
 }finally{await act(async()=>root.unmount());container.remove()}
});
