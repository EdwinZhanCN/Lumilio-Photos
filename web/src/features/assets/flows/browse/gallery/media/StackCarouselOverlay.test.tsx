import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import type { Asset, StackByAssetResponse } from "@/lib/assets/types";
import StackCarouselOverlay from "./StackCarouselOverlay";

// The heavy AssetViewer carousel is the boundary of this component test; a stub
// exposes the slide props so the real stack-resolution chain (stack details +
// per-member asset fetches) can be exercised through MSW and asserted on.
vi.mock("../../../viewer/AssetViewer", () => ({
  default: ({ initialSlide, slideIndex }: { initialSlide: number; slideIndex?: number }) => (
    <div
      data-testid="fullscreen-carousel"
      data-initial-slide={initialSlide}
      data-slide-index={slideIndex ?? ""}
    />
  ),
}));

const asset = {
  asset_id: "cover",
  original_filename: "cover.jpg",
  stack: { stack_id: "stack-1", stack_size: 2, stack_cover: true },
} as Asset;

function serveStack() {
  const stackResponse = {
    asset_id: "cover",
    stack: {
      stack_id: "stack-1",
      member_count: 2,
      members: [
        { primary_asset_id: "cover", position: 0 },
        { primary_asset_id: "member", position: 1 },
      ],
    },
  } satisfies StackByAssetResponse;

  worker.use(
    http.get("*/api/v1/assets/:id/stack", () => HttpResponse.json(stackResponse)),
    // The cover member reuses the current asset; only the other member is fetched.
    http.get("*/api/v1/assets/member", () =>
      HttpResponse.json({ asset_id: "member", original_filename: "member.jpg" }),
    ),
  );
}

describe("StackCarouselOverlay", () => {
  it("opens on the matched member when a focus asset is provided", async () => {
    serveStack();
    const screen = await renderWithProviders(
      <StackCarouselOverlay asset={asset} focusAssetId="member" open onClose={vi.fn()} />,
      { router: false },
    );

    const carousel = screen.getByTestId("fullscreen-carousel");
    await expect.element(carousel).toHaveAttribute("data-initial-slide", "1");
    await expect.element(carousel).toHaveAttribute("data-slide-index", "1");
    expect(carousel.element().parentElement).toBe(document.body);
  });

  it("falls back to the cover when no matched member focus is provided", async () => {
    serveStack();
    const screen = await renderWithProviders(
      <StackCarouselOverlay asset={asset} open onClose={vi.fn()} />,
      { router: false },
    );

    const carousel = screen.getByTestId("fullscreen-carousel");
    await expect.element(carousel).toHaveAttribute("data-initial-slide", "0");
    await expect.element(carousel).toHaveAttribute("data-slide-index", "0");
  });
});
