import { expect, test } from "vite-plus/test";
import { page } from "vitest/browser";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { StorageDiagnostic } from "@/features/repositories";
import { CapacityMap } from "./CapacityMap";
import "@/styles/App.css";

const item = {
  kind: "external",
  rawName: "Fixture disk",
  path: "/fixture",
  entityType: "storage_location",
  target_id: "fixture-disk",
  capacity_known: true,
  total_bytes: 1000,
  available_bytes: 380,
  safety_margin_bytes: 50,
  writable_budget_bytes: 330,
} satisfies StorageDiagnostic;

test("rectangle pixel areas match capacity with no gaps, including a tiny reserve", async () => {
  await page.viewport(900, 700);
  const screen = await renderWithProviders(
    <div style={{ width: 800 }}>
      <CapacityMap item={item} />
    </div>,
  );
  const map = screen.getByRole("group", { name: t("monitor.storage.capacityHeading") });
  const box = map.element().getBoundingClientRect();
  for (const [label, fraction] of [
    [t("monitor.storage.statUsed"), 0.62],
    [t("monitor.storage.uploadBudget"), 0.33],
    [t("monitor.storage.safetyReserve"), 0.05],
  ] as const) {
    const area = map.getByRole("button", { name: label }).element().getBoundingClientRect();
    expect((area.width * area.height) / (box.width * box.height)).toBeCloseTo(fraction, 3);
  }
  const reserve = map.getByRole("button", { name: t("monitor.storage.safetyReserve") });
  // The small rectangle does not expand to fit copy; the adjacent legend stays usable.
  expect(getComputedStyle(reserve.element().firstElementChild!).display).toBe("none");
  await reserve.click();
  await expect
    .element(screen.getByText(t("monitor.storage.reserveTarget", { target: "50 Bytes" })))
    .toBeVisible();
});

test("reserve shortfall cannot draw more than available; unknown capacity has no graph", async () => {
  const screen = await renderWithProviders(
    <CapacityMap item={{ ...item, available_bytes: 20, writable_budget_bytes: 0 }} />,
  );
  const map = screen.getByRole("group", { name: t("monitor.storage.capacityHeading") });
  await expect
    .element(map.getByRole("button", { name: t("monitor.storage.safetyReserve") }))
    .toHaveAccessibleName(`${t("monitor.storage.safetyReserve")}: 20 Bytes`);
  await expect
    .element(map.getByRole("button", { name: t("monitor.storage.uploadBudget") }))
    .not.toBeInTheDocument();
  await screen.rerender(<CapacityMap item={{ ...item, capacity_known: false }} />);
  await expect.element(screen.getByText(t("monitor.storage.capacityUnknown"))).toBeVisible();
  await expect.element(map).not.toBeInTheDocument();
});
