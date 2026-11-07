import { resolverReport } from "./fixtures/resolvers";
import { correlationReport } from "./fixtures/correlations";
import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after,test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { MonitoringProvider } from "../context/MonitoringContext";
import AnomalyFeed from "../components/analytics/AnomalyFeed";
import TrendChart from "../components/analytics/TrendChart";
import { incidentReport } from "./fixtures/incidents";
import { anomalyReport,episode } from "./fixtures/anomalies";
import { availability,report } from "./fixtures/analytics";
after(()=>dom.window.close());

test("anomaly feed shows severity and recovery and paginates with real controls",async(t)=>{
 const calls:string[]=[];
 t.mock.method(globalThis,"fetch",async(input:RequestInfo|URL)=>{
  const u=new URL(String(input)); if(u.pathname==="/api/dns/resolvers") return new Response(JSON.stringify(resolverReport(u.searchParams.get("range")??"15m"))); if(u.pathname==="/api/correlations") return new Response(JSON.stringify(correlationReport()));calls.push(u.toString());
  if(u.pathname==="/api/incidents") return new Response(JSON.stringify(incidentReport(u.searchParams.get("range") ?? "15m")));
  if(u.pathname==="/api/anomalies"){
   const old=episode(true);old.id="ffffffffffffffffffffffffffffffff-0";old.target="old:443";
   const next=anomalyReport("15m",[u.searchParams.has("before")?old:episode()]);
   if(u.searchParams.get("type")==="dns"){next.anomalies[0].type="dns";next.anomalies[0].target="example.test"}
   if(!u.searchParams.has("before")){next.has_more=true;next.next_cursor="opaque-next-page"}
   return new Response(JSON.stringify(next));
  }
  return new Response(JSON.stringify(u.pathname==="/api/analytics"?report():u.pathname==="/api/availability"?availability():{count:0,snapshots:[]}));
 });
 const container=document.createElement("div");document.body.appendChild(container);const root=createRoot(container);
 try{
  await act(async()=>root.render(<MonitoringProvider><AnomalyFeed/></MonitoringProvider>));
  assert.match(container.textContent??"",/critical/);assert.match(container.textContent??"",/Open/);assert.match(container.textContent??"",/Baseline median/);
  const older=[...container.querySelectorAll("button")].find(b=>b.textContent==="Older episodes");assert.ok(older&&!older.disabled);
  await act(async()=>{older.click()});
  assert.match(container.textContent??"",/old:443/);assert.match(container.textContent??"",/Recovered/);assert.ok(calls.some(v=>v.includes("before=opaque-next-page")));
  const previous=[...container.querySelectorAll("button")].find(b=>b.textContent==="Previous page");assert.ok(previous);
  await act(async()=>{previous.click()});assert.match(container.textContent??"",/alpha:443/);
  const select=container.querySelector<HTMLSelectElement>('select[aria-label="Anomaly feed probe type"]');assert.ok(select);
  await act(async()=>{select.value="dns";select.dispatchEvent(new Event("change",{bubbles:true}))});
  assert.match(container.textContent??"",/example.test/);assert.ok(calls.some(v=>v.includes("type=dns")));
 }finally{await act(async()=>root.unmount());container.remove()}
});

test("latency charts render onset markers separately from bucket average latency",async()=>{
 const container=document.createElement("div");document.body.appendChild(container);const root=createRoot(container);
 const points=report().current.trends[0].points;
 points[59]={...points[59],anomaly_count:2,anomaly_latency_ms:100};
 try{
  await act(async()=>root.render(<TrendChart title="Latency trend" description="Test measurements" metric="avg_latency_ms" points={points}/>));
  assert.match(container.textContent??"",/maximum anomaly-onset latency/);
  assert.ok(container.querySelector('circle[fill="#F87171"]'),"onsets need visible red markers");
 }finally{await act(async()=>root.unmount());container.remove()}
});
