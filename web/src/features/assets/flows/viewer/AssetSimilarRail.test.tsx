import { useState } from "react";
import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset } from "@/lib/assets/types";
import type { components } from "@/lib/http-commons/schema";
import { AssetSimilarRail } from "./AssetSimilarRail";

vi.mock("../browse/gallery/media/MediaThumbnail", () => ({
  default: ({ asset }: { asset: Asset }) => <div>{asset.original_filename ?? asset.asset_id}</div>,
}));

type CapabilitiesResponse = components["schemas"]["dto.CapabilitiesResponseDTO"];
type SearchAssetsResponse = components["schemas"]["dto.SearchAssetsResponseDTO"];

const queryAssetId = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa";
const inCarouselId = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb";
const outsideId = "cccccccc-cccc-cccc-cccc-cccccccccccc";

const capabilities: CapabilitiesResponse = {
  ml: {
    discovery_state: "healthy",
    tasks: {
      semantic_image_embed: { enabled: true, available: true },
    },
  },
  llm: { availability: "disabled", agent_enabled: false, configured: false },
};

const searchResponse: SearchAssetsResponse = {
  result_items: [
    {
      type: "media_item",
      id: `media:${inCarouselId}`,
      media_item: {
        media_item_id: inCarouselId,
        primary_asset: {
          asset_id: inCarouselId,
          original_filename: "in-carousel.jpg",
          type: "PHOTO",
        },
      },
    },
    {
      type: "media_item",
      id: `media:${outsideId}`,
      media_item: {
        media_item_id: outsideId,
        primary_asset: {
          asset_id: outsideId,
          original_filename: "outside.jpg",
          type: "PHOTO",
        },
      },
    },
  ],
  results_total_visible: 2,
  top_items: [],
};

function serve(onSearch: () => void) {
  worker.use(
    http.get("*/api/v1/capabilities", () => HttpResponse.json(capabilities)),
    http.get("*/api/v1/assets/:id/thumbnail", () => new HttpResponse(null, { status: 204 })),
    http.post("*/api/v1/assets/search", () => {
      onSearch();
      return HttpResponse.json(searchResponse);
    }),
  );
}

const carouselAssets: Asset[] = [
  { asset_id: queryAssetId, original_filename: "query.jpg", type: "PHOTO" },
  { asset_id: inCarouselId, original_filename: "in-carousel.jpg", type: "PHOTO" },
];

describe("AssetSimilarRail", () => {
  it("does not fetch until opened", async () => {
    let searches = 0;
    serve(() => {
      searches += 1;
    });
    await renderWithProviders(
      <AssetSimilarRail
        open={false}
        queryAssetId={queryAssetId}
        carouselAssets={carouselAssets}
        onNavigate={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(searches).toBe(0);
  });

  it("fetches when opened and exposes a main-library see-all link", async () => {
    let searches = 0;
    serve(() => {
      searches += 1;
    });
    const screen = await renderWithProviders(
      <AssetSimilarRail
        open
        queryAssetId={queryAssetId}
        carouselAssets={carouselAssets}
        onNavigate={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    await expect.element(screen.getByText("in-carousel.jpg")).toBeVisible();
    expect(searches).toBe(1);
    await expect
      .element(screen.getByRole("link", { name: t("assets.mediaViewer.similarSeeAll") }))
      .toHaveAttribute("href", `/assets?similar=${queryAssetId}`);
  });

  it("does not prefetch when the closed rail's query asset changes", async () => {
    let searches = 0;
    serve(() => {
      searches += 1;
    });
    function Harness() {
      const [id, setId] = useState(queryAssetId);
      return (
        <>
          <button type="button" onClick={() => setId(outsideId)}>
            next-slide
          </button>
          <AssetSimilarRail
            open={false}
            queryAssetId={id}
            carouselAssets={carouselAssets}
            onNavigate={vi.fn()}
            onClose={vi.fn()}
          />
        </>
      );
    }
    const screen = await renderWithProviders(<Harness />);
    await screen.getByRole("button", { name: "next-slide" }).click();
    expect(searches).toBe(0);
  });
});
