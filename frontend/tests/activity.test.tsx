import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import ActivityTable from "../components/dashboard/ActivityTable";
after(() => dom.window.close());
test("activity has separate fields, bounded pages and working probe filters", async () => {
  const rows = Array.from({ length: 55 }, (_, i) => ({
    type: i % 2 === 0 ? ("tcp" as const) : ("dns" as const),
    target: `target-${i}`,
    timestamp: new Date(1_700_000_000_000 + i * 1000).toISOString(),
    success: i !== 54,
    latency_ms: i,
    error: i === 54 ? "connection refused" : "",
  }));
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<ActivityTable results={rows} />));
    assert.equal(container.querySelectorAll("tbody tr").length, 50);
    assert.match(container.textContent ?? "", /connection refused/);
    assert.match(container.textContent ?? "", /Failed/);
    assert.match(container.textContent ?? "", /54.00 ms/);
    const buttons = container.querySelectorAll<HTMLButtonElement>("button");
    await act(async () => buttons[1].click());
    assert.equal(container.querySelectorAll("tbody tr").length, 5);
    assert.match(container.textContent ?? "", /Page 2 of 2/);
    const select = container.querySelector<HTMLSelectElement>("select");
    assert.ok(select);
    await act(async () => {
      select.value = "tcp";
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    assert.equal(container.querySelectorAll("tbody tr").length, 28);
    assert.match(container.textContent ?? "", /Page 1 of 1/);
    assert.ok(buttons[1].disabled);
  } finally {
    await act(async () => root.unmount());
  }
});
