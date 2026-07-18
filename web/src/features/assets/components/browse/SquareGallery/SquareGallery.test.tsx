import { cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import { createBrowseGroupsFromAssets } from "../../../utils/browseItems";
import SquareGallery from "./SquareGallery";

vi.mock("../../media/MediaThumbnail", () => ({
  default: ({ asset }: { asset: Asset }) => <div>{asset.asset_id}</div>,
}));
vi.mock("../../media/StackedThumbnail", () => ({
  default: ({ asset }: { asset: Asset }) => <div>{asset.asset_id}</div>,
}));
vi.mock("../../../hooks/useSelection", () => ({
  useOptionalKeyboardSelection: () => ({
    enabled: false,
    handleClick: vi.fn(),
    handleKeyDown: vi.fn(),
    isSelected: () => false,
  }),
}));
vi.mock("../../../hooks/useGalleryInfiniteScroll", async (importOriginal) => {
  const original = await importOriginal<typeof import("../../../hooks/useGalleryInfiniteScroll")>();
  return { ...original, useGalleryInfiniteScroll: () => ({ supportsIntersectionObserver: true }) };
});
vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { count?: number }) => String(options?.count ?? _key),
    i18n: { language: "en", resolvedLanguage: "en" },
  }),
}));

beforeEach(() => {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("SquareGallery large-library window", () => {
  it("keeps mounted media bounded for a 10,000-item fixture", async () => {
    const assets = Array.from(
      { length: 10_000 },
      (_, index): Asset => ({
        asset_id: `asset-${index}`,
        original_filename: `asset-${index}.jpg`,
      }),
    );
    const { container } = render(
      <SquareGallery
        browseGroups={createBrowseGroupsFromAssets(assets)}
        openCarousel={vi.fn()}
        onLoadMore={vi.fn()}
        hasMore={false}
        isLoadingMore={false}
        columns={4}
      />,
    );

    await waitFor(() => {
      expect(container.querySelector("[data-gallery-total='10000']")).not.toBeNull();
    });
    expect(container.querySelectorAll("[data-asset-id]").length).toBeLessThanOrEqual(48);
  });
});
