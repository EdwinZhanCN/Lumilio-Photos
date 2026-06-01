import { cleanup, render, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import PhotoPicker from "./PhotoPicker";

const mocks = vi.hoisted(() => ({
  useCurrentAssetsView: vi.fn(),
  useSelection: vi.fn(),
}));

vi.mock("@/contexts/WorkerProvider", () => ({
  WorkerProvider: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("@/features/assets/AssetsProvider", () => ({
  AssetsProvider: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("@/features/assets/selectors", () => ({
  useSortBy: () => "date_captured",
  useUIActions: () => ({
    setSortBy: vi.fn(),
    setSearchQuery: vi.fn(),
  }),
  useFilterActions: () => ({
    resetFilters: vi.fn(),
    batchUpdateFilters: vi.fn(),
  }),
  useSelectionActions: () => ({
    clear: vi.fn(),
    setEnabled: vi.fn(),
  }),
}));

vi.mock("@/features/assets/hooks/useAssetsView", () => ({
  useCurrentAssetsView: mocks.useCurrentAssetsView,
}));

vi.mock("@/features/assets/hooks/useSelection", () => ({
  useSelection: mocks.useSelection,
}));

vi.mock("@/features/assets/components/page/SquareGallery/SquareGallery", () => ({
  default: () => <div>square-gallery</div>,
}));

vi.mock("@/features/assets/components/shared/AssetsPageHeader", () => ({
  default: () => <div>assets-page-header</div>,
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) =>
      options?.defaultValue ?? _key,
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const createAsset = (
  assetId: string,
  overrides: Partial<Asset> = {},
): Asset =>
  ({
    asset_id: assetId,
    original_filename: `${assetId}.jpg`,
    ...overrides,
  }) as Asset;

describe("PhotoPicker", () => {
  it("resolves stack selections to the representative asset id", async () => {
    const onSelect = vi.fn();

    mocks.useCurrentAssetsView.mockReturnValue({
      assets: [],
      browseGroups: [
        {
          key: "flat:all",
          items: [
            {
              type: "stack",
              id: "stack:stack-1",
              stackId: "stack-1",
              representative: createAsset("cover", {
                stack: {
                  stack_id: "stack-1",
                  stack_size: 2,
                  stack_cover: true,
                },
              }),
              assets: [
                createAsset("cover", {
                  stack: {
                    stack_id: "stack-1",
                    stack_size: 2,
                    stack_cover: true,
                  },
                }),
                createAsset("member", {
                  stack: {
                    stack_id: "stack-1",
                    stack_size: 2,
                    stack_cover: false,
                  },
                }),
              ],
            },
          ],
        },
      ],
      browseItems: [
        {
          type: "stack",
          id: "stack:stack-1",
          stackId: "stack-1",
          representative: createAsset("cover", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: true,
            },
          }),
          assets: [
            createAsset("cover", {
              stack: {
                stack_id: "stack-1",
                stack_size: 2,
                stack_cover: true,
              },
            }),
            createAsset("member", {
              stack: {
                stack_id: "stack-1",
                stack_size: 2,
                stack_cover: false,
              },
            }),
          ],
        },
      ],
      browseAssets: [
        createAsset("cover", {
          stack: {
            stack_id: "stack-1",
            stack_size: 2,
            stack_cover: true,
          },
        }),
      ],
      isLoading: false,
      isLoadingMore: false,
      fetchMore: vi.fn(),
      hasMore: false,
      viewKey: "picker-view",
    });
    mocks.useSelection.mockReturnValue({
      enabled: true,
      selectedIds: new Set(["stack:stack-1"]),
      selectedCount: 1,
    });

    render(<PhotoPicker scopeId="test-scope" onSelect={onSelect} />);

    await waitFor(() => {
      expect(onSelect).toHaveBeenCalledWith("cover");
    });
  });
});
