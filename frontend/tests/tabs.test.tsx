import { dom } from "./dom-setup";
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { act } from "react";
import { createRoot } from "react-dom/client";
import DataTabs from "../components/dashboard/DataTabs";
after(() => dom.window.close());
test("data tabs isolate panels and support arrow, home and end navigation", async () => {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  try {
    await act(async () =>
      root.render(
        <DataTabs
          label="Measurements"
          tabs={[
            { id: "latency", label: "Latency", content: <h2>Only latency</h2> },
            { id: "jitter", label: "Jitter", content: <h2>Only jitter</h2> },
            { id: "targets", label: "Targets", content: <h2>Only targets</h2> },
          ]}
        />,
      ),
    );
    const tabs = container.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    assert.equal(container.querySelectorAll('[role="tabpanel"]').length, 1);
    assert.match(container.textContent ?? "", /Only latency/);
    assert.doesNotMatch(container.textContent ?? "", /Only jitter/);
    await act(async () => {
      tabs[0].focus();
      tabs[0].dispatchEvent(
        new dom.window.KeyboardEvent("keydown", {
          key: "ArrowRight",
          bubbles: true,
        }),
      );
    });
    assert.equal(document.activeElement, tabs[1]);
    assert.equal(tabs[1].getAttribute("aria-selected"), "true");
    assert.match(container.textContent ?? "", /Only jitter/);
    await act(async () =>
      tabs[1].dispatchEvent(
        new dom.window.KeyboardEvent("keydown", { key: "End", bubbles: true }),
      ),
    );
    assert.equal(document.activeElement, tabs[2]);
    assert.match(container.textContent ?? "", /Only targets/);
    await act(async () =>
      tabs[2].dispatchEvent(
        new dom.window.KeyboardEvent("keydown", { key: "Home", bubbles: true }),
      ),
    );
    assert.equal(tabs[0].tabIndex, 0);
    assert.equal(tabs[1].tabIndex, -1);
    assert.equal(
      container
        .querySelector('[role="tabpanel"]')
        ?.getAttribute("aria-labelledby"),
      tabs[0].id,
    );
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});
