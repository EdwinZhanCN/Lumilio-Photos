import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import type { components } from "@/lib/http-commons/schema";
import type { LumenRuntime } from "../../api/useLumenRuntime";
import { CapabilitiesMonitor } from "./CapabilitiesMonitor";

type CapabilitiesResponse = components["schemas"]["dto.CapabilitiesResponseDTO"];

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
      await expect.element(screen.getByText("Loading...", { exact: true })).toBeVisible();
    } finally {
      releaseSnapshots();
    }
    await expect.element(screen.getByText("Lumen discovery is healthy")).toBeVisible();
  });

  it("distinguishes a healthy zero-node scan from discovery failure", async () => {
    serve();
    const screen = await renderWithProviders(<CapabilitiesMonitor />);

    await expect.element(screen.getByText("Lumen discovery is healthy")).toBeVisible();
    await expect
      .element(screen.getByRole("heading", { name: "No validated Lumen Hubs are advertised" }))
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

    await expect.element(screen.getByText("Lumen discovery is degraded")).toBeVisible();
    await expect.element(screen.getByText(/Discovery scan timed out/)).toBeVisible();
    await expect.element(screen.getByText(/2 consecutive failures/)).toBeVisible();
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

    await expect.element(screen.getByText("lab-node-1", { exact: true })).toBeVisible();
    await expect.element(screen.getByText("Capability exchange pending")).toBeVisible();
    await expect.element(screen.getByText("pending", { exact: true }).first()).toBeVisible();
    await expect.element(screen.getByText("ready", { exact: true })).toBeVisible();
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

    await expect.element(screen.getByText("incompatible", { exact: true }).first()).toBeVisible();
    await expect.element(screen.getByText("Protocol version is incompatible")).toBeVisible();
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

    await expect.element(screen.getByText("active", { exact: true })).toBeVisible();
    await expect
      .element(screen.getByText("Image Semantic Analysis", { exact: true }))
      .toBeVisible();
    await expect.element(screen.getByText("Person Recognition", { exact: true })).toBeVisible();
    await expect.element(screen.getByText("siglip / semantic_image_embed")).toBeVisible();
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
      .element(screen.getByRole("heading", { name: "No validated Lumen Hubs are advertised" }))
      .toBeVisible();

    recovered = true;
    await screen.getByRole("button", { name: "Refresh status" }).click();
    await expect.element(screen.getByText("recovered-node", { exact: true })).toBeVisible();
    await expect.element(screen.getByText("active", { exact: true })).toBeVisible();
  });
});
