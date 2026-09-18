import "@/styles/App.css";
import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import type { LumenRuntime } from "../../api/useLumenRuntime";
import { CapabilitiesMonitor } from "./CapabilitiesMonitor";

type CapabilitiesResponse = components["schemas"]["dto.CapabilitiesResponseDTO"];

/**
 * Build a substring matcher from app copy without embedding the copy itself.
 * Diagnostic cells combine a typed label with a failure count, so the locator
 * must stay a partial match while remaining key-resolved.
 */
function copyPattern(value: string): RegExp {
  return new RegExp(value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
}

const publicCapabilities: CapabilitiesResponse = {
  ml: {
    discovery_state: "healthy",
    discovered_node_count: 0,
    active_node_count: 0,
    connecting_node_count: 0,
    unavailable_node_count: 0,
    pending_node_count: 0,
    incompatible_node_count: 0,
    tasks: {
      semantic_image_embed: { enabled: true, available: false },
      semantic_text_embed: { enabled: true, available: false },
      bioclip_classify: { enabled: false, available: false },
      ocr: { enabled: true, available: false },
      face_recognition: { enabled: true, available: false },
    },
  },
  llm: {
    availability: "disabled",
    agent_enabled: false,
    configured: false,
  },
};

const healthyRuntime: LumenRuntime = {
  captured_at: "2026-08-12T12:00:00Z",
  discovery_state: "healthy",
  counts: {
    discovered: 0,
    active: 0,
    connecting: 0,
    unavailable: 0,
    pending: 0,
    incompatible: 0,
  },
  backends: [
    {
      source: "mdns",
      state: "healthy",
      last_scan_succeeded_at: "2026-08-12T11:59:59Z",
      matched_count: 0,
      rejected_count: 0,
      consecutive_failures: 0,
      last_outcome: "success",
    },
  ],
  nodes: [],
};

function serve(runtime = healthyRuntime, capabilities = publicCapabilities) {
  worker.use(
    http.get("*/api/v1/capabilities", () => HttpResponse.json(capabilities)),
    http.get("*/api/v1/admin/lumen/runtime", () => HttpResponse.json(runtime)),
  );
}

describe("CapabilitiesMonitor", () => {
  it("renders a stable loading state while both snapshots are pending", async () => {
    let releaseSnapshots!: () => void;
    const snapshotsPending = new Promise<void>((resolve) => {
      releaseSnapshots = resolve;
    });
    worker.use(
      http.get("*/api/v1/capabilities", async () => {
        await snapshotsPending;
        return HttpResponse.json(publicCapabilities);
      }),
      http.get("*/api/v1/admin/lumen/runtime", async () => {
        await snapshotsPending;
        return HttpResponse.json(healthyRuntime);
      }),
    );
    const screen = await renderWithProviders(<CapabilitiesMonitor />);

    try {
      await expect.element(screen.getByText(t("common.loading"), { exact: true })).toBeVisible();
    } finally {
      releaseSnapshots();
    }
    await expect.element(screen.getByText(t("monitor.capabilities.noNodes"))).toBeVisible();
  });

  it("distinguishes a healthy zero-node scan from discovery failure", async () => {
    serve();
    const screen = await renderWithProviders(<CapabilitiesMonitor />);

    await expect.element(screen.getByText(t("monitor.capabilities.noNodes"))).toBeVisible();
    await expect
      .element(screen.getByText(t("monitor.capabilities.noNodes"), { exact: true }))
      .toBeVisible();
    await expect.element(screen.getByText("mdns", { exact: true })).toBeVisible();
  });

  it("shows a degraded backend with a typed diagnostic", async () => {
    serve({
      ...healthyRuntime,
      discovery_state: "degraded",
      backends: [
        {
          source: "mdns",
          state: "degraded",
          last_scan_succeeded_at: "2026-08-12T11:59:59Z",
          matched_count: 0,
          rejected_count: 0,
          last_error_code: "query_timed_out",
          consecutive_failures: 2,
          last_outcome: "timed_out",
        },
      ],
    });
    const screen = await renderWithProviders(<CapabilitiesMonitor />);

    await expect
      .element(screen.getByText(t("monitor.capabilities.discovery.degraded")))
      .toBeVisible();
    await expect
      .element(screen.getByText(copyPattern(t("monitor.capabilities.errors.queryTimedOut"))))
      .toBeVisible();
    await expect
      .element(
        screen.getByText(copyPattern(t("monitor.capabilities.consecutiveFailures", { count: 2 }))),
      )
      .toBeVisible();
  });

  it("shows a discovered node as capability-pending before routing", async () => {
    serve({
      ...healthyRuntime,
      counts: { ...healthyRuntime.counts, discovered: 1, pending: 1 },
      nodes: [
        {
          id: "lab-node-1",
          endpoint: "192.168.1.20:5866",
          sources: ["mdns"],
          transport: "ready",
          compatibility: "pending",
          last_observed_at: "2026-08-12T12:00:00Z",
          tasks: [],
        },
      ],
    });
    const screen = await renderWithProviders(<CapabilitiesMonitor />);
    await expect
      .element(screen.getByRole("region", { name: t("monitor.snapshot.selection") }))
      .not.toBeInTheDocument();
    await screen.getByRole("group", { name: t("monitor.capabilities.orbitLabel") }).hover();
    await screen
      .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
      .getByRole("button", { name: "lab-node-1", exact: false })
      .click();
    await screen.getByText(t("monitor.capabilities.nodeDetails"), { exact: true }).click();

    await expect
      .element(
        screen
          .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
          .getByText("lab-node-1", { exact: true }),
      )
      .toBeVisible();
    await expect
      .element(
        screen
          .getByRole("region", { name: t("monitor.snapshot.selection") })
          .getByText(t("monitor.capabilities.capabilityPending")),
      )
      .toBeVisible();
    await expect
      .element(screen.getByText(t("monitor.capabilities.states.pending"), { exact: true }).first())
      .toBeVisible();
    await expect
      .element(
        screen
          .getByRole("region", { name: t("monitor.snapshot.selection") })
          .getByText(t("monitor.capabilities.states.ready"), { exact: true }),
      )
      .toBeVisible();
  });

  it("shows incompatible nodes without presenting their tasks as available", async () => {
    serve({
      ...healthyRuntime,
      counts: { ...healthyRuntime.counts, discovered: 1, incompatible: 1 },
      nodes: [
        {
          id: "legacy-node",
          endpoint: "192.168.1.21:5866",
          sources: ["mdns"],
          transport: "ready",
          compatibility: "incompatible",
          error_code: "protocol_incompatible",
          tasks: [],
        },
      ],
    });
    const screen = await renderWithProviders(<CapabilitiesMonitor />);
    await expect
      .element(screen.getByRole("region", { name: t("monitor.snapshot.selection") }))
      .not.toBeInTheDocument();
    await screen.getByRole("group", { name: t("monitor.capabilities.orbitLabel") }).hover();
    await screen
      .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
      .getByRole("button", { name: "legacy-node", exact: false })
      .click();
    await screen.getByText(t("monitor.capabilities.nodeDetails"), { exact: true }).click();

    await expect
      .element(
        screen.getByText(t("monitor.capabilities.states.incompatible"), { exact: true }).first(),
      )
      .toBeVisible();
    await expect
      .element(
        screen
          .getByRole("region", { name: t("monitor.snapshot.selection") })
          .getByText(t("monitor.capabilities.errors.protocolIncompatible")),
      )
      .toBeVisible();
  });

  it("shows a compatible transport as active with canonical capability labels", async () => {
    serve(
      {
        ...healthyRuntime,
        counts: { ...healthyRuntime.counts, discovered: 1, active: 1 },
        nodes: [
          {
            id: "active-node",
            endpoint: "192.168.1.22:5866",
            sources: ["mdns"],
            transport: "ready",
            compatibility: "compatible",
            tasks: [
              { service: "siglip", task: "semantic_image_embed" },
              { service: "face", task: "face_recognition" },
            ],
          },
        ],
      },
      {
        ...publicCapabilities,
        ml: {
          ...publicCapabilities.ml,
          discovered_node_count: 1,
          active_node_count: 1,
          tasks: {
            semantic_image_embed: { enabled: true, available: true },
            semantic_text_embed: { enabled: true, available: true },
            bioclip_classify: { enabled: false, available: false },
            ocr: { enabled: true, available: false },
            face_recognition: { enabled: true, available: true },
          },
        },
      },
    );
    const screen = await renderWithProviders(<CapabilitiesMonitor />);
    await expect
      .element(screen.getByRole("region", { name: t("monitor.snapshot.selection") }))
      .not.toBeInTheDocument();
    await screen.getByRole("group", { name: t("monitor.capabilities.orbitLabel") }).hover();
    await screen
      .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
      .getByRole("button", { name: "active-node", exact: false })
      .click();
    await screen.getByText(t("monitor.capabilities.nodeDetails"), { exact: true }).click();

    await expect
      .element(
        screen
          .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
          .getByText(t("monitor.capabilities.states.active"), { exact: true }),
      )
      .toBeVisible();
    await expect
      .element(screen.getByText(t("settings.aiSettings.taskNames.semantic"), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByText(t("settings.aiSettings.taskNames.face"), { exact: true }))
      .toBeVisible();
    await expect
      .element(
        screen
          .getByRole("region", { name: t("monitor.snapshot.selection") })
          .getByText("siglip / semantic_image_embed", { exact: false }),
      )
      .toBeVisible();
  });

  it("refreshes an expired empty snapshot into a recovered active node", async () => {
    let recovered = false;
    worker.use(
      http.get("*/api/v1/capabilities", () => HttpResponse.json(publicCapabilities)),
      http.get("*/api/v1/admin/lumen/runtime", () =>
        HttpResponse.json(
          recovered
            ? {
                ...healthyRuntime,
                counts: { ...healthyRuntime.counts, discovered: 1, active: 1 },
                nodes: [
                  {
                    id: "recovered-node",
                    endpoint: "192.168.1.24:5866",
                    sources: ["mdns"],
                    transport: "ready",
                    compatibility: "compatible",
                    tasks: [],
                  },
                ],
              }
            : healthyRuntime,
        ),
      ),
    );
    const screen = await renderWithProviders(<CapabilitiesMonitor />);
    await expect
      .element(screen.getByText(t("monitor.capabilities.noNodes"), { exact: true }))
      .toBeVisible();

    recovered = true;
    await screen.getByRole("button", { name: t("settings.serverSettings.refresh") }).click();
    await expect
      .element(
        screen
          .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
          .getByText("recovered-node", { exact: true }),
      )
      .toBeVisible();
    await expect
      .element(
        screen
          .getByRole("group", { name: t("monitor.capabilities.orbitLabel") })
          .getByText(t("monitor.capabilities.states.active"), { exact: true }),
      )
      .toBeVisible();
  });
});

it.each(["disabled", "starting"] as const)(
  "keeps %s discovery explicit with no nodes",
  async (state) => {
    serve({ ...healthyRuntime, discovery_state: state });
    const screen = await renderWithProviders(<CapabilitiesMonitor />);
    await expect
      .element(screen.getByText(t(`monitor.capabilities.discovery.${state}`), { exact: true }))
      .toBeVisible();
    await expect
      .element(screen.getByText(t("monitor.capabilities.noNodes"), { exact: true }))
      .toBeVisible();
  },
);

it("marks a failed runtime refresh stale while retaining prior endpoint diagnostics", async () => {
  let failed = false;
  worker.use(
    http.get("*/api/v1/capabilities", () => HttpResponse.json(publicCapabilities)),
    http.get("*/api/v1/admin/lumen/runtime", () =>
      failed
        ? HttpResponse.json({ title: "Fixture failure" }, { status: 500 })
        : HttpResponse.json(healthyRuntime),
    ),
  );
  const screen = await renderWithProviders(<CapabilitiesMonitor />);
  await expect
    .element(screen.getByText(t("monitor.capabilities.noNodes"), { exact: true }))
    .toBeVisible();
  failed = true;
  await screen
    .getByRole("button", { name: t("settings.serverSettings.refresh"), exact: true })
    .click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.snapshot.stale"));
  await expect
    .element(screen.getByText(t("monitor.capabilities.noNodes"), { exact: true }))
    .toBeVisible();
});
