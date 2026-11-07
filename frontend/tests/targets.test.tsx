import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import TargetManager from "../components/targets/TargetManager";
import { parseTarget, parseTargets, targetRequest } from "../lib/targets";
import type { MonitoringTarget } from "../types/targets";
after(() => dom.window.close());
const token = "test-token-for-ui-tests-at-least-32-characters";
const fixture = (): MonitoringTarget => ({ id: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", type: "tcp", address: "local.test:443", enabled: true, interval_ms: 5000, timeout_ms: 3000 });
function button(container: HTMLElement, text: string) { const node = [...container.querySelectorAll("button")].find(b => b.textContent === text); assert.ok(node, `missing ${text}`); return node }
async function input(container: HTMLElement, label: string, value: string) {
  const node = container.querySelector<HTMLInputElement>(`input[aria-label="${label}"]`); assert.ok(node);
  await act(async () => { Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, "value")?.set?.call(node, value); node.dispatchEvent(new Event("input", { bubbles: true })) });
}
async function submit(container: HTMLElement, selector = "form") { const form = container.querySelector(selector); assert.ok(form); await act(async () => { form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })) }) }

test("target parsing validates bounds, uniqueness and response shape", () => {
  assert.equal(parseTargets({ count: 0, targets: [] }).length, 0); assert.equal(parseTarget(fixture()).address, "local.test:443");
  for (const bad of [{ ...fixture(), id: "bad" }, { ...fixture(), timeout_ms: 6000 }, { ...fixture(), enabled: "true" }, { ...fixture(), interval_ms: 4999 }, { ...fixture(), type: "icmp" }]) assert.throws(() => parseTarget(bad));
  assert.throws(() => parseTargets({ count: 2, targets: [fixture(), fixture()] }));
});

test("target page authenticates, adds, edits, toggles, removes and locks without persisting the token", async t => {
  let saved = [fixture()]; const calls: { method: string; authorization: string }[] = [];
  t.mock.method(globalThis, "fetch", async (_input: RequestInfo | URL, options?: RequestInit) => {
    const method = options?.method ?? "GET"; calls.push({ method, authorization: new Headers(options?.headers).get("Authorization") ?? "" });
    if (method === "GET") return new Response(JSON.stringify({ count: saved.length, targets: saved }));
    if (method === "DELETE") { saved = saved.filter(v => v.id !== String(_input).split("/").at(-1)); return new Response(JSON.stringify({ deleted: true })) }
    const body = JSON.parse(String(options?.body)); const value = { ...body, id: method === "POST" ? "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" : String(_input).split("/").at(-1) } as MonitoringTarget;
    saved = [...saved.filter(v => v.id !== value.id), value]; return new Response(JSON.stringify(value));
  });
  const container = document.createElement("div"); document.body.appendChild(container); const root = createRoot(container);
  try {
    await act(async () => root.render(<TargetManager />)); assert.equal(calls.length, 0);
    await input(container, "Admin token", token); await submit(container);
    assert.match(container.textContent ?? "", /local.test:443/); assert.ok(calls.every(c => c.authorization === `Bearer ${token}`)); assert.equal(window.localStorage.length, 0); assert.equal(window.sessionStorage.length, 0);
    await act(async () => button(container, "Disable").click()); assert.match(container.textContent ?? "", /Disabled/);
    await act(async () => button(container, "Edit").click()); await input(container, "Probe interval seconds", "10"); await input(container, "Probe timeout seconds", "5"); await submit(container, 'form[aria-label="Target editor"]');
    assert.match(container.textContent ?? "", /Every 10s/); assert.match(container.textContent ?? "", /timeout 5s/);
    const select = container.querySelector<HTMLSelectElement>('select[aria-label="Target probe type"]'); assert.ok(select); await act(async () => { select.value = "dns"; select.dispatchEvent(new Event("change", { bubbles: true })) });
    await input(container, "Target address", "example.test"); await submit(container, 'form[aria-label="Target editor"]'); assert.match(container.textContent ?? "", /DNS · example.test/);
    const remove = container.querySelector<HTMLButtonElement>('button[aria-label="Remove example.test"]'); assert.ok(remove); await act(async () => remove.click()); assert.match(container.textContent ?? "", /Historical records will be retained/);
    await act(async () => button(container, "Confirm removal").click()); assert.doesNotMatch(container.textContent ?? "", /DNS · example.test/); assert.match(container.textContent ?? "", /Historical measurements and incidents remain/);
    await act(async () => button(container, "Lock management").click()); assert.ok(container.querySelector('input[aria-label="Admin token"]')); assert.doesNotMatch(container.textContent ?? "", /local.test:443/);
  } finally { await act(async () => root.unmount()); container.remove() }
});

test("authentication errors remain visible and empty configurations offer an add form", async t => {
  let denied = true;
  t.mock.method(globalThis, "fetch", async () => new Response(JSON.stringify(denied ? { error: "A valid admin token is required" } : { count: 0, targets: [] }), { status: denied ? 401 : 200 }));
  const container = document.createElement("div"); const root = createRoot(container);
  try {
    await act(async () => root.render(<TargetManager />)); await input(container, "Admin token", token); await submit(container); assert.match(container.querySelector('[role="alert"]')?.textContent ?? "", /valid admin token/);
    denied = false; await submit(container); assert.match(container.textContent ?? "", /No targets configured/); assert.ok(container.querySelector('form[aria-label="Target editor"]'));
  } finally { await act(async () => root.unmount()) }
});

test("API errors, cancellation and private authorization headers are handled", async t => {
  const controller = new AbortController();
  t.mock.method(globalThis, "fetch", async (_url: RequestInfo | URL, options?: RequestInit) => { assert.equal(options?.signal, controller.signal); assert.equal(new Headers(options?.headers).get("Authorization"), `Bearer ${token}`); assert.equal(options?.cache, "no-store"); return new Response(JSON.stringify({ error: "Target management is disabled" }), { status: 503 }) });
  await assert.rejects(targetRequest(token, "GET", null, controller.signal), /management is disabled/);
});
