import { cleanup, render, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import PhotoPicker from "./PhotoPicker";

const mocks = vi.hoisted(() => ({
  useCurrentAssetsView: vi.fn(),
  useSelection: vi.fn(),
  AssetsProvider: vi.fn((props: { children: ReactNode }) => props.children),
  AssetsPageHeader: vi.fn(
    (_props: { lockedFilterFields?: readonly string[] }) => "assets-page-header",
  ),
  resetFilters: vi.fn(),
  batchUpdateFilters: vi.fn(),
  setSearchQuery: vi.fn(),
  setSelectionEnabled: vi.fn(),
  clearSelection: vi.fn(),
}));

vi.mock("@/contexts/WorkerProvider", () => ({
  WorkerProvider: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("../state/AssetsProvider", () => ({
  AssetsProvider: mocks.AssetsProvider,
}));

vi.mock("../state/selectors", () => ({
  useSortBy: () => "date_captured",
  useUIActions: () => ({
    setSortBy: vi.fn(),
    setSearchQuery: mocks.setSearchQuery,
  }),
  useFilterActions: () => ({
    resetFilters: mocks.resetFilters,
    batchUpdateFilters: mocks.batchUpdateFilters,
  }),
  useSelectionActions: () => ({
    clear: mocks.clearSelection,
    setEnabled: mocks.setSelectionEnabled,
  }),
}));

vi.mock("../api/useAssetsView", () => ({
  useCurrentAssetsView: mocks.useCurrentAssetsView,
}));

vi.mock("../hooks/useSelection", () => ({
  useSelection: mocks.useSelection,
}));

vi.mock("../components/browse/SquareGallery/SquareGallery", () => ({
  default: () => <div>square-gallery</div>,
}));

vi.mock("../components/browse/AssetsPageHeader", () => ({
  default: mocks.AssetsPageHeader,
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key,
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const createAsset = (assetId: string, overrides: Partial<Asset> = {}): Asset =>
  ({
    asset_id: assetId,
    original_filename: `${assetId}.jpg`,
    ...overrides,
  }) as Asset;

describe("PhotoPicker", () => {
  it("locks the media type filter by default", async () => {
    mocks.useCurrentAssetsView.mockReturnValue({
      assets: [],
      browseGroups: [],
      browseItems: [],
      browseAssets: [],
      isLoading: false,
      isLoadingMore: false,
      fetchMore: vi.fn(),
      hasMore: false,
      viewKey: "picker-view",
    });
    mocks.useSelection.mockReturnValue({
      enabled: true,
      selectedIds: new Set(),
      selectedCount: 0,
    });

    render(<PhotoPicker scopeId="test-scope" onSelect={vi.fn()} />);

    await waitFor(() => {
      expect(mocks.batchUpdateFilters).toHaveBeenCalledWith({
        enabled: true,
        type: "PHOTO",
        raw: undefined,
      });
    });
    expect(mocks.AssetsPageHeader.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        lockedFilterFields: ["type"],
        hiddenBulkActions: [
          "set-rating",
          "set-liked",
          "stack-selected",
          "add-tags",
          "add-to-album",
          "download",
          "delete-assets",
        ],
      }),
    );
  });

  it("passes additional locked filters and initial raw constraint", async () => {
    mocks.useCurrentAssetsView.mockReturnValue({
      assets: [],
      browseGroups: [],
      browseItems: [],
      browseAssets: [],
      isLoading: false,
      isLoadingMore: false,
      fetchMore: vi.fn(),
      hasMore: false,
      viewKey: "picker-view",
    });
    mocks.useSelection.mockReturnValue({
      enabled: true,
      selectedIds: new Set(),
      selectedCount: 0,
    });

    render(
      <PhotoPicker
        scopeId="test-scope"
        onSelect={vi.fn()}
        initialFilters={{ raw: false }}
        lockedFields={["type", "raw"]}
      />,
    );

    await waitFor(() => {
      expect(mocks.batchUpdateFilters).toHaveBeenCalledWith({
        enabled: true,
        type: "PHOTO",
        raw: false,
      });
    });
    expect(mocks.AssetsProvider.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        initialState: {
          filters: {
            enabled: true,
            type: "PHOTO",
            raw: false,
          },
        },
      }),
    );
    expect(mocks.AssetsPageHeader.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        lockedFilterFields: ["type", "raw"],
        hiddenBulkActions: [
          "set-rating",
          "set-liked",
          "stack-selected",
          "add-tags",
          "add-to-album",
          "download",
          "delete-assets",
        ],
      }),
    );
  });

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
