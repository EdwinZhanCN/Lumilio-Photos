import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { AssetDeleteImpact } from "./AssetDeleteImpact";

describe("Asset delete confirmation", () => {
  it("previews every selected physical Asset and the configured retention", async () => {
    const requests: unknown[] = [];
    const impact = {
      assets: 2,
      files: 3,
      bytes: 300,
      repositories: [
        { id: "repo-a", name: "First" },
        { id: "repo-b", name: "Second" },
      ],
      retention_days: 17,
    } satisfies components["schemas"]["dto.AssetDeleteImpactDTO"];
    worker.use(
      http.post("*/api/v1/assets/delete-impact", async ({ request }) => {
        requests.push(await request.json());
        return HttpResponse.json(impact);
      }),
    );
    const ready = vi.fn();
    const screen = await renderWithProviders(
      <AssetDeleteImpact assetIds={["a", "b"]} onReady={ready} />,
    );
    await expect
      .element(screen.getByText(t("assets.lifecycle.retention", { days: 17 })))
      .toBeVisible();
    await expect
      .element(
        screen.getByText(t("assets.lifecycle.impact", { assets: 2, files: 3, bytes: "300 Bytes" })),
      )
      .toBeVisible();
    await expect
      .element(screen.getByText(t("assets.lifecycle.repositories", { names: "First, Second" })))
      .toBeVisible();
    expect(requests).toEqual([{ asset_ids: ["a", "b"] }]);
    expect(ready).toHaveBeenLastCalledWith(true);
  });
  it("keeps confirmation disabled when the preview fails", async () => {
    worker.use(
      http.post("*/api/v1/assets/delete-impact", () =>
        HttpResponse.json(
          {
            type: "about:blank",
            status: 500,
            instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
          },
          { status: 500 },
        ),
      ),
    );
    const ready = vi.fn();
    const screen = await renderWithProviders(
      <AssetDeleteImpact assetIds={["a"]} onReady={ready} />,
    );
    await expect.element(screen.getByRole("alert")).toBeVisible();
    expect(ready).toHaveBeenLastCalledWith(false);
  });
});
