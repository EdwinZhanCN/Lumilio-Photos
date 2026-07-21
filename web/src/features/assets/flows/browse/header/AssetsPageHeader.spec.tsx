import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset } from "@/lib/assets/types";
import type { BrowseItem } from "../../../types";
import { AssetBrowserScope } from "../selection/AssetBrowserScope";
import AssetsPageHeader from "./AssetsPageHeader";

// Header-only flow spec: no gallery/WASM. The selection is seeded through the
// real AssetBrowserScope; browse-scope and album lookups run against MSW. The
// subject is the header's bulk-action orchestration (context resolution and the
// confirmation gate), so no child components are mocked.
const browseItems: BrowseItem[] = [
  { type: "asset", id: "asset:asset-1", asset: { asset_id: "asset-1" } as Asset },
];

function serveHeaderBootstrap() {
  worker.use(
    http.get("*/api/v1/assets/indexing/repositories", () =>
      HttpResponse.json({ repositories: [{ id: "repo-1", name: "Repo 1" }] }),
    ),
    http.get("*/api/v1/albums", () => HttpResponse.json({ albums: [] })),
  );
}

const HIDE_ALL = [
  "set-rating",
  "set-liked",
  "stack-selected",
  "add-tags",
  "add-to-album",
  "download",
  "delete-assets",
] as const;

function renderHeader(node: React.ReactNode) {
  serveHeaderBootstrap();
  return renderWithProviders(
    <AssetBrowserScope
      scopeId="header-spec"
      initialSelection={{ enabled: true, selectedIds: ["asset:asset-1"] }}
    >
      {node}
    </AssetBrowserScope>,
  );
}

describe("AssetsPageHeader bulk actions", () => {
  it("runs a custom action with the resolved selection context", async () => {
    const onRun = vi.fn();
    const screen = await renderHeader(
      <AssetsPageHeader
        sortBy="date_captured"
        onSortByChange={vi.fn()}
        filter={{}}
        onFiltersChange={vi.fn()}
        title="Assets"
        browseItems={browseItems}
        hiddenBulkActions={[...HIDE_ALL]}
        bulkActions={[{ id: "custom-action", label: "Custom action", onRun }]}
      />,
    );

    await screen.getByText("Custom action").first().click();

    await vi.waitFor(() => {
      expect(onRun).toHaveBeenCalledWith(
        expect.objectContaining({
          selectedItemCount: 1,
          affectedAssetCount: 1,
          selectedAssetIds: ["asset-1"],
        }),
      );
    });
  });

  it("gates a confirmation-required action behind its dialog", async () => {
    const onRun = vi.fn();
    const screen = await renderHeader(
      <AssetsPageHeader
        sortBy="date_captured"
        onSortByChange={vi.fn()}
        filter={{}}
        onFiltersChange={vi.fn()}
        title="Assets"
        browseItems={browseItems}
        hiddenBulkActions={[...HIDE_ALL]}
        bulkActions={[
          {
            id: "confirm-action",
            label: "Confirm action",
            requiresConfirmation: true,
            confirmationTitle: "Run confirmed action?",
            onRun,
          },
        ]}
      />,
    );

    await screen.getByText("Confirm action").first().click();
    await expect.element(screen.getByText("Run confirmed action?")).toBeVisible();
    expect(onRun).not.toHaveBeenCalled();

    await screen.getByRole("button", { name: t("common.confirm"), exact: true }).click();
    await vi.waitFor(() => expect(onRun).toHaveBeenCalledOnce());
  });
});
