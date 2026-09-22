import { afterEach, expect, test } from "vite-plus/test";
import { cdp, page } from "vitest/browser";
import { renderWithProviders } from "@test/render";
import { http, HttpResponse, worker } from "@test/msw";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { MLMonitor } from "./MLMonitor";
import { StatMonitor } from "./StatMonitor";
import { CapabilitiesMonitor } from "./CapabilitiesMonitor";
import "@/styles/App.css";

const runtime = {
  discovery_state: "degraded",
  counts: { discovered: 6, active: 4, incompatible: 1, unavailable: 1 },
  nodes: Array.from({ length: 6 }, (_, index) => ({
    id: `fixture-hub-${index}`,
    endpoint: `192.0.2.${index + 1}:5866`,
    transport: index === 1 ? "unavailable" : "ready",
    compatibility: index === 2 ? "incompatible" : "compatible",
    error_code: index === 2 ? "protocol_incompatible" : undefined,
    tasks: [
      {
        service: index === 5 ? "ocr" : "siglip",
        task: index === 5 ? "ocr" : "semantic_image_embed",
      },
    ],
  })),
  backends: [
    {
      source: "mdns",
      state: "healthy",
      last_scan_succeeded_at: "2026-09-18T04:00:23Z",
      next_scan_at: "2026-09-18T04:00:53Z",
      matched_count: 6,
      rejected_count: 0,
      consecutive_failures: 0,
      last_outcome: "success",
    },
  ],
} satisfies components["schemas"]["dto.LumenRuntimeDTO"];
const capability = {
  ml: {
    discovery_state: "degraded",
    tasks: {
      semantic_image_embed: { enabled: true, available: true },
      semantic_text_embed: { enabled: true, available: false },
      face_recognition: { enabled: false, available: false },
      ocr: { enabled: true, available: true },
      bioclip_classify: { enabled: true, available: false },
    },
  },
  llm: {
    agent_enabled: false,
    configured: true,
    provider: "fixture-provider",
    model_name: "fixture-model",
  },
} satisfies components["schemas"]["dto.CapabilitiesResponseDTO"];
function serve() {
  worker.use(
    http.get("*/api/v1/assets/indexing/stats", () =>
      HttpResponse.json({
        photo_total: 2000,
        video_total: 240,
        reindex_jobs: 3,
        tasks: {
          semantic: { total_count: 2000, indexed_count: 1640, queued_jobs: 32 },
          video_semantic: { total_count: 240, indexed_count: 144, queued_jobs: 8 },
          ocr: { total_count: 2000, indexed_count: 1880, queued_jobs: 12 },
          face: { total_count: 2000, indexed_count: 1520, queued_jobs: 32 },
          bioclip: { total_count: 200, indexed_count: 90 },
        },
      }),
    ),
    http.get("*/api/v1/admin/monitor/processing", () =>
      HttpResponse.json({
        deliveries: { completed: 1200, discarded: 2, queues: [] },
        processing: {
          pending_assets: 248,
          failed_assets: 3,
          pending_repositories: 4,
          failed_repositories: 0,
          pending_projections: 12,
          failed_projections: 1,
          pending_operations: 7,
          failed_operations: 0,
          pending_analysis_assets: 32,
          failed_analysis_assets: 0,
          pending_reindex_requests: 3,
        },
      }),
    ),
    http.get("*/api/v1/capabilities", () => HttpResponse.json(capability)),
    http.get("*/api/v1/admin/lumen/runtime", () => HttpResponse.json(runtime)),
  );
}
afterEach(async () => {
  await cdp().send("Emulation.clearDeviceMetricsOverride");
  await page.viewport(1280, 900);
});

for (const theme of ["light", "dark"])
  for (const width of [390, 800, 1280]) {
    test(`three primary visuals stay readable at ${width}px in ${theme}`, async () => {
      await cdp().send("Emulation.setDeviceMetricsOverride", {
        width: 1600,
        height: 2400,
        deviceScaleFactor: 1,
        mobile: false,
      });
      await page.viewport(width, 1000);
      serve();
      const wrap = (view: React.ReactNode) => (
        <main
          data-theme={theme}
          className="min-h-screen bg-base-100 p-6 text-base-content"
          style={{ fontFamily: "system-ui, sans-serif" }}
        >
          {view}
        </main>
      );
      let screen = await renderWithProviders(wrap(<MLMonitor />));
      await expect.element(screen.getByText("82%", { exact: true })).toBeVisible();
      const fields = [...screen.container.querySelectorAll(".monitor-weave > section")];
      const tops = fields.map((field) => Math.round(field.getBoundingClientRect().top));
      expect(tops.filter((top) => top === tops[0])).toHaveLength(
        width === 1280 ? 5 : width === 800 ? 3 : 2,
      );
      const capture = async (name: string) => {
        expect(screen.container.scrollWidth).toBeLessThanOrEqual(width);
        if (import.meta.env.VITE_MONITOR_SCREENSHOTS) {
          await page.viewport(width, Math.max(1000, document.body.scrollHeight));
          await page.screenshot({
            path: `../../../../../.local/monitor-qa/${name}-${theme}-${width}.png`,
          });
          await page.viewport(width, 1000);
        }
      };
      await capture("ml");
      await screen.unmount();
      screen = await renderWithProviders(wrap(<StatMonitor />));
      const files = screen.getByRole("region", {
        name: t("monitor.processing.pendingAssets"),
        exact: true,
      });
      await expect.element(files.getByText("248", { exact: true })).toBeVisible();
      if (matchMedia("(prefers-reduced-motion: reduce)").matches) {
        await expect
          .element(files.element().querySelector("figure"))
          .toHaveAttribute("data-animated", "false");
        expect(screen.container.querySelector("canvas")).toBeNull();
      } else {
        await expect
          .element(files.element().querySelector("figure"))
          .toHaveAttribute("data-loaded", "true");
        // Only the files tray animates; every other kind of work is a list row.
        await expect
          .poll(() => {
            const canvases = [...screen.container.querySelectorAll("canvas")];
            return (
              canvases.length === 1 &&
              canvases.every((canvas) =>
                canvas
                  .getContext("2d")
                  ?.getImageData(0, 0, canvas.width, canvas.height)
                  .data.some((v, i) => i % 4 === 3 && v > 0),
              )
            );
          })
          .toBe(true);
      }
      await capture("processing");
      await screen.unmount();
      screen = await renderWithProviders(wrap(<CapabilitiesMonitor />));
      const orbit = screen.getByRole("group", { name: t("monitor.capabilities.orbitLabel") });
      await expect
        .element(orbit.getByRole("button", { name: "fixture-hub-0", exact: false }))
        .toBeVisible();
      for (const node of screen.container.querySelectorAll(".monitor-node")) {
        const bounds = node.getBoundingClientRect();
        expect(bounds.left).toBeGreaterThanOrEqual(0);
        expect(bounds.right).toBeLessThanOrEqual(width);
      }
      await capture("capabilities");
    });
  }

test("the center selects discovery and reopens the inspector after viewing a node", async () => {
  serve();
  const screen = await renderWithProviders(<CapabilitiesMonitor />);
  const orbit = screen.getByRole("group", { name: t("monitor.capabilities.orbitLabel") });
  await expect.element(screen.getByText("mdns", { exact: true })).toBeVisible();
  await expect.element(screen.getByRole("table")).not.toBeInTheDocument();
  await expect
    .element(screen.getByRole("button", { name: t("settings.aiSettings.taskNames.ocr") }))
    .not.toBeInTheDocument();
  await screen.getByRole("button", { name: t("common.next"), exact: true }).click();
  await orbit.hover();
  await orbit.getByRole("button", { name: "fixture-hub-5", exact: false }).click();
  await expect
    .element(orbit.getByRole("button", { name: "fixture-hub-5", exact: false }))
    .toHaveAttribute("aria-pressed", "true");
  await screen.getByText(t("monitor.capabilities.nodeDetails"), { exact: true }).click();
  const details = screen.getByRole("region", { name: t("monitor.snapshot.selection") });
  await expect.element(details).toHaveTextContent("ocr / ocr");
  await expect.element(details.getByText("192.0.2.6:5866", { exact: true })).toBeVisible();
  await screen
    .getByRole("complementary")
    .getByRole("button", { name: "fixture-hub-5", exact: false })
    .click();
  await expect.element(details).not.toBeInTheDocument();
  const center = orbit.getByRole("button", { name: t("app.name"), exact: true });
  await center.click();
  await expect.element(center).toHaveAttribute("aria-pressed", "true");
  await expect
    .element(screen.getByText("fixture-provider", { exact: false }))
    .not.toBeInTheDocument();
  await expect.element(details).not.toBeInTheDocument();
  await expect.element(screen.getByText("mdns", { exact: true })).toBeVisible();
});

test("reduced motion renders static trays without loading a canvas", async () => {
  await cdp().send("Emulation.setEmulatedMedia", {
    features: [{ name: "prefers-reduced-motion", value: "reduce" }],
  });
  try {
    serve();
    const screen = await renderWithProviders(
      <main data-theme="dark" className="bg-base-100 p-6 text-base-content">
        <StatMonitor />
      </main>,
    );
    const files = screen.getByRole("region", {
      name: t("monitor.processing.pendingAssets"),
      exact: true,
    });
    await expect.element(files.getByText("248", { exact: true })).toBeVisible();
    expect(matchMedia("(prefers-reduced-motion: reduce)").matches).toBe(true);
    expect(screen.container.querySelector("canvas")).toBeNull();
    const sheet = screen.container.querySelector(".monitor-tray-sheet")!;
    expect(getComputedStyle(sheet).transitionDuration).toBe("0s");
  } finally {
    await cdp().send("Emulation.setEmulatedMedia", { features: [] });
  }
});
