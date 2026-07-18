import { cleanup, render, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import AlbumDetails from "./AlbumDetails";
import type { AssetsBulkActionContext } from "@/lib/assets/bulkActions";

const mocks = vi.hoisted(() => ({
  AssetsProvider: vi.fn((props: { children: ReactNode }) => props.children),
  AssetsGalleryPage: vi.fn((_props: unknown) => "assets-gallery-page"),
  invalidateQueries: vi.fn(),
  removeAssetFromAlbum: vi.fn(),
  rebuildBioClip: vi.fn(),
  showMessage: vi.fn(),
}));

vi.mock("react-router-dom", () => ({
  useParams: () => ({ albumId: "42" }),
}));

vi.mock("@/contexts/WorkerProvider", () => ({
  WorkerProvider: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("@/features/assets", () => ({
  AssetsProvider: mocks.AssetsProvider,
  AssetsGalleryPage: mocks.AssetsGalleryPage,
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({
    invalidateQueries: mocks.invalidateQueries,
  }),
}));

vi.mock("@/features/repositories", () => ({
  useBrowseScope: () => ({
    scopedRepositoryId: undefined,
  }),
}));

vi.mock("@/features/notifications", () => ({
  useMessage: () => mocks.showMessage,
}));

vi.mock("@/lib/i18n.tsx", () => ({
  getCurrentLanguage: () => "en",
  useI18n: () => ({
    i18n: { language: "en", resolvedLanguage: "en" },
    t: (key: string, options?: { defaultValue?: string; count?: number }) =>
      options?.defaultValue ?? key,
  }),
}));

vi.mock("@/lib/http-commons/queryClient", () => ({
  $api: {
    useQuery: vi.fn(() => ({
      data: {
        album_id: 42,
        album_name: "Summer",
        album_type: "manual",
        asset_count: 3,
        created_at: "2026-06-18T00:00:00Z",
      },
      isLoading: false,
    })),
    useMutation: vi.fn((method: string, path: string) => {
      if (method === "delete" && path === "/api/v1/albums/{id}/assets/{assetId}") {
        return { mutateAsync: mocks.removeAssetFromAlbum };
      }
      if (method === "post" && path === "/api/v1/albums/{id}/bioclip/rebuild") {
        return { mutateAsync: mocks.rebuildBioClip, isPending: false };
      }
      return { mutateAsync: vi.fn(), isPending: false };
    }),
  },
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("AlbumDetails bulk actions", () => {
  it("adds a remove-from-current-album action and hides asset delete", async () => {
    mocks.removeAssetFromAlbum.mockResolvedValue({});
    mocks.invalidateQueries.mockResolvedValue(undefined);

    render(<AlbumDetails />);

    const props = mocks.AssetsGalleryPage.mock.calls[0][0] as {
      hiddenBulkActions: string[];
      bulkActions: (context: AssetsBulkActionContext) => unknown[];
    };

    expect(props.hiddenBulkActions).toEqual(["delete-assets"]);

    const context: AssetsBulkActionContext = {
      selectedItemCount: 2,
      affectedAssetCount: 2,
      selectedAssetIds: ["asset-a", "asset-b"],
      selectedAssets: [],
      clearSelection: vi.fn(),
    };
    const actions = props.bulkActions(context) as Array<{
      id: string;
      onRun: (context: AssetsBulkActionContext) => Promise<void>;
    }>;

    expect(actions.map((action) => action.id)).toContain("share-selected");

    const removeAction = actions.find((action) => action.id === "remove-from-current-album");
    expect(removeAction).toBeDefined();

    await removeAction!.onRun(context);

    await waitFor(() => {
      expect(mocks.removeAssetFromAlbum).toHaveBeenCalledTimes(2);
    });
    expect(mocks.removeAssetFromAlbum).toHaveBeenCalledWith({
      params: { path: { id: 42, assetId: "asset-a" } },
      body: {},
    });
    expect(mocks.removeAssetFromAlbum).toHaveBeenCalledWith({
      params: { path: { id: 42, assetId: "asset-b" } },
      body: {},
    });
    expect(context.clearSelection).toHaveBeenCalled();
    expect(mocks.invalidateQueries).toHaveBeenCalled();
    expect(mocks.showMessage).toHaveBeenCalledWith(
      "success",
      "collections.albumDetails.bulkActions.removeFromAlbum.success",
    );
  });
});
