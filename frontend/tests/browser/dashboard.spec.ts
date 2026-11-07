import { expect, test, type Page, type Route } from "@playwright/test";
import { chartScenario } from "../fixtures/chart-scenario";
import { availability } from "../fixtures/analytics";
import type { TimeRange } from "../../lib/monitoring";
import { anomalyReport, episode } from "../fixtures/anomalies";
import { incidentReport, failureIncident } from "../fixtures/incidents";
import { correlationReport, correlationEvent } from "../fixtures/correlations";
import { resolverReport } from "../fixtures/resolvers";
import { navigation } from "../../lib/navigation";
async function mockMonitoring(page: Page) {
  await page.route("http://localhost:8080/api/**", async (route) => {
    const url = new URL(route.request().url());
    const range = url.searchParams.get("range") ?? "15m";
    const body =
      url.pathname === "/api/analytics"
        ? chartScenario(range as TimeRange)
        : url.pathname === "/api/availability"
          ? availability()
          : url.pathname === "/api/anomalies"
            ? anomalyReport(range, [episode()])
            : url.pathname === "/api/incidents"
              ? incidentReport(range, [failureIncident()])
              : url.pathname === "/api/correlations"
                ? correlationReport(range, [correlationEvent()])
                : url.pathname === "/api/dns/resolvers"
                  ? resolverReport(range, true)
                  : { count: 0, snapshots: [] };
    await route.fulfill({
      json: body,
      headers: { "Access-Control-Allow-Origin": "http://localhost:3000" },
    });
  });
  await page.routeWebSocket("ws://localhost:8080/api/ws", (socket) =>
    socket.close(),
  );
}
test.beforeEach(async ({ page }) => {
  await mockMonitoring(page);
});

test("leaving an export cancels it and keeps the cached page usable", async ({
  page,
}) => {
  let pending: Route | undefined;
  let downloads = 0;
  page.on("download", () => downloads++);
  await page.route("http://localhost:8080/api/export/**", (route) => {
    pending = route;
  });
  await page.goto("/exports");
  await page.getByRole("button", { name: "Download CSV", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Cancel export" }),
  ).toBeVisible();
  await expect.poll(() => Boolean(pending)).toBe(true);
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "Overview", exact: true })
    .click();
  await expect(page).toHaveURL("http://localhost:3000/");
  await expect(
    page.getByRole("heading", { name: "Overview", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "Exports", exact: true })
    .click();
  await expect(page).toHaveURL("http://localhost:3000/exports");
  await expect(
    page.getByRole("button", { name: "Download CSV", exact: true }),
  ).toBeEnabled();
  expect(downloads).toBe(0);
  await pending?.abort().catch(() => {});
});
test("separate pages and analytics tabs remain usable on desktop", async ({
  page,
}) => {
  const failures: string[] = [];
  page.on("pageerror", (error) => failures.push(error.message));
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Overview", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Failure incident timeline" }),
  ).toHaveCount(0);
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "TCP analytics", exact: true })
    .click();
  await expect(
    page.getByRole("tab", { name: "Latency", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  for (const label of [
    "Percentiles",
    "Distribution",
    "Jitter",
    "Targets",
    "Period comparison",
  ]) {
    await page.getByRole("tab", { name: label, exact: true }).click();
    await expect(
      page.getByRole("tab", { name: label, exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("tabpanel")).toHaveCount(1);
  }
  await page.getByRole("tab", { name: "Latency", exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Percentiles" })).toBeFocused();
  await page.getByRole("tab", { name: "Latency", exact: true }).click();
  await expect(
    page.locator('[role="tabpanel"] .recharts-line-curve'),
  ).toHaveAttribute("d", /C/);
  await page.screenshot({
    path: "test-results/tcp-desktop.png",
    fullPage: true,
  });
  for (const item of navigation.filter(
    (item) => !["/", "/tcp"].includes(item.href),
  )) {
    await page
      .getByRole("navigation", { name: "Main navigation" })
      .getByRole("link", { name: item.label, exact: true })
      .click();
    await expect(page).toHaveURL(new RegExp(`${item.href}$`));
    await expect(page.locator("h1:visible")).toBeVisible();
  }
  expect(failures).toEqual([]);
});
test("mobile navigation and tables stay inside the viewport", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/tcp");
  await expect(page.getByLabel("Dashboard page")).toBeVisible();
  for (const href of [
    "/tcp",
    "/dns",
    "/health",
    "/resolvers",
    "/activity",
    "/exports",
    "/incidents",
  ]) {
    await page.getByLabel("Dashboard page").selectOption(href);
    await expect(page).toHaveURL(new RegExp(`${href}$`));
    await expect
      .poll(() =>
        page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth + 1,
        ),
      )
      .toBe(true);
  }
  await page.getByLabel("Dashboard page").selectOption("/tcp");
  await page.getByRole("tab", { name: "Targets", exact: true }).click();
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth + 1,
      ),
    )
    .toBe(true);
  await page.screenshot({
    path: "test-results/tcp-mobile.png",
    fullPage: true,
  });
});

test("target credentials are cleared when navigating away from a cached page", async ({
  page,
}) => {
  await page.route("http://localhost:8080/api/targets", (route) =>
    route.fulfill({
      json: { count: 0, targets: [] },
      headers: { "Access-Control-Allow-Origin": "http://localhost:3000" },
    }),
  );
  await page.goto("/targets");
  await page
    .getByLabel("Admin token", { exact: true })
    .fill("test-only-admin-credential-at-least-32-characters");
  await page.getByRole("button", { name: "Unlock target management" }).click();
  await expect(
    page.getByRole("button", { name: "Lock management", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "Overview", exact: true })
    .click();
  await expect(page).toHaveURL("http://localhost:3000/");
  await expect(
    page.getByRole("heading", { name: "Overview", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "Targets", exact: true })
    .click();
  await expect(page).toHaveURL("http://localhost:3000/targets");
  await expect(page.getByLabel("Admin token", { exact: true })).toHaveValue("");
  await expect(
    page.getByRole("button", { name: "Unlock target management" }),
  ).toBeEnabled();
});
