import { JSDOM } from "jsdom";

export const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: "http://localhost:3000" });
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
// jsdom has no layout engine. Supply a deterministic chart container size.
dom.window.HTMLElement.prototype.getBoundingClientRect = () => new dom.window.DOMRect(0, 0, 640, 250);
Object.defineProperty(globalThis, "navigator", { value: dom.window.navigator, configurable: true });
Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { value: true, configurable: true });
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

class TestSocket {
  static OPEN = 1;
  static CONNECTING = 0;
  readyState = 0;
  close() { this.readyState = 3 }
}
globalThis.WebSocket = TestSocket as unknown as typeof WebSocket;
