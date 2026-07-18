import { cleanup, render, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import PhotoPicker from "./PhotoPicker";

const mocks = vi.hoisted(() => ({
  useAssetBrowser: vi.fn(),
  useAssetSelection: vi.fn(),
  AssetBrowserScope: vi.fn((props: { children: ReactNode }) => props.children),
  AssetsPageHeader: vi.fn((_props: Record<string, unknown>) => "assets-page-header"),
  setSelectionEnabled: vi.fn(),
  clearSelection: vi.fn(),
}));

vi.mock("@/contexts/WorkerProvider", () => ({
  WorkerProvider: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("../flows/browse/selection/AssetBrowserScope", () => ({
  AssetBrowserScope: mocks.AssetBrowserScope,
}));

vi.mock("../flows/browse/selection/useAssetSelection", () => ({
  useAssetSelectionActions: () => ({
    clear: mocks.clearSelection,
    setEnabled: mocks.setSelectionEnabled,
  }),
  useAssetSelection: mocks.useAssetSelection,
}));

vi.mock("../flows/browse/useAssetBrowser", () => ({
  useAssetBrowser: mocks.useAssetBrowser,
}));

vi.mock("../flows/browse/gallery/SquareGallery/SquareGallery", () => ({
  default: () => <div>square-gallery</div>,
}));

vi.mock("../flows/browse/header/AssetsPageHeader", () => ({
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
    mocks.useAssetBrowser.mockReturnValue({
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
    mocks.useAssetSelection.mockReturnValue({
      enabled: true,
      selectedIds: new Set(),
      selectedCount: 0,
    });

    render(<PhotoPicker scopeId="test-scope" onSelect={vi.fn()} />);

    expect(mocks.useAssetBrowser).toHaveBeenCalledWith(
      expect.objectContaining({
        constraint: { type: "PHOTO" },
        userFilter: {},
      }),
    );
    expect(mocks.AssetsPageHeader.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        constraint: { type: "PHOTO" },
        filter: {},
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
    mocks.useAssetBrowser.mockReturnValue({
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
    mocks.useAssetSelection.mockReturnValue({
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

    expect(mocks.useAssetBrowser).toHaveBeenCalledWith(
      expect.objectContaining({
        constraint: { type: "PHOTO", raw: false },
        userFilter: {},
      }),
    );
    expect(mocks.AssetBrowserScope.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        initialSelection: { selectionMode: "single" },
      }),
    );
    expect(mocks.AssetsPageHeader.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        constraint: { type: "PHOTO", raw: false },
        filter: {},
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

    mocks.useAssetBrowser.mockReturnValue({
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
    mocks.useAssetSelection.mockReturnValue({
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
