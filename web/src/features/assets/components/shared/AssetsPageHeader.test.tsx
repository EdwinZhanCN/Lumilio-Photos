import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import AssetsPageHeader from "./AssetsPageHeader";
import type { Asset } from "@/lib/assets/types";
import type { BrowseItem } from "@/features/assets/types/assets.type";

const mocks = vi.hoisted(() => ({
  selectionClear: vi.fn(),
  setSelectionEnabled: vi.fn(),
  batchUpdateFilters: vi.fn(),
  showMessage: vi.fn(),
  scanRepositories: vi.fn(),
  createStack: vi.fn(),
  listAlbums: vi.fn(),
}));

vi.mock("@/components/PageHeader", () => ({
  default: ({ children, title }: { children: ReactNode; title: string }) => (
    <header>
      <h1>{title}</h1>
      {children}
    </header>
  ),
}));

vi.mock("@/features/assets/components/page/FilterTool/FilterTool", () => ({
  default: () => <div data-testid="filter-tool" />,
}));

vi.mock("@/features/assets/hooks/useSelection", () => ({
  useSelection: () => ({
    enabled: true,
    selectedIds: new Set(["asset:asset-1"]),
    selectedCount: 1,
    setEnabled: mocks.setSelectionEnabled,
    clear: mocks.selectionClear,
  }),
  useBulkAssetOperations: () => ({
    bulkDelete: vi.fn(),
    bulkDownload: vi.fn(),
    bulkAddToAlbum: vi.fn(),
    bulkAddTags: vi.fn(),
    bulkUpdateRating: vi.fn(),
    bulkSetLike: vi.fn(),
  }),
}));

vi.mock("@/features/assets/selectors", () => ({
  useFilterState: () => ({ enabled: false }),
  useFilterActions: () => ({
    batchUpdateFilters: mocks.batchUpdateFilters,
  }),
}));

vi.mock("@/hooks/util-hooks/useMessage", () => ({
  useMessage: () => mocks.showMessage,
}));

vi.mock("@/features/repositories", () => ({
  BrowseScopeSelect: () => null,
  useBrowseScope: () => ({
    repositories: [{ id: "repo-1", name: "Repo 1" }],
    browseRepositoryId: "",
    scopedRepositoryId: undefined,
    selectedRepository: undefined,
    scopeLabel: "All libraries",
    setBrowseRepositoryId: vi.fn(),
    getRepositoryLabel: (repo: { name?: string }) => repo.name ?? "Repo",
  }),
}));

vi.mock("@/features/manage/hooks/useRepositoryScan", () => ({
  useRepositoryScan: () => ({
    scanRepositories: mocks.scanRepositories,
    isScanning: false,
  }),
}));

vi.mock("@/features/assets/hooks/useStackActions", () => ({
  useStackActions: () => ({
    createStack: mocks.createStack,
    isCreatingStack: false,
  }),
}));

vi.mock("@/lib/http-commons/queryClient", () => ({
  $api: {
    useQuery: () => ({ data: { albums: [] }, isPending: false }),
    useMutation: () => ({
      mutateAsync: mocks.listAlbums,
    }),
  },
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string, options?: { defaultValue?: string; count?: number }) =>
      options?.defaultValue ?? key,
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const createAsset = (assetId: string): Asset =>
  ({
    asset_id: assetId,
    original_filename: `${assetId}.jpg`,
  }) as Asset;

const browseItems: BrowseItem[] = [
  {
    type: "asset",
    id: "asset:asset-1",
    asset: createAsset("asset-1"),
  },
];

describe("AssetsPageHeader bulk actions", () => {
  it("renders custom actions while hiding requested default actions", async () => {
    const onRun = vi.fn();

    render(
      <AssetsPageHeader
        sortBy="date_captured"
        onSortByChange={vi.fn()}
        title="Assets"
        browseItems={browseItems}
        hiddenBulkActions={[
          "set-rating",
          "set-liked",
          "stack-selected",
          "add-tags",
          "add-to-album",
          "download",
          "delete-assets",
        ]}
        bulkActions={[
          {
            id: "custom-action",
            label: "Custom action",
            onRun,
          },
        ]}
      />,
    );

    expect(screen.queryByText("Set Rating")).not.toBeInTheDocument();
    expect(screen.queryByText("Add to Album")).not.toBeInTheDocument();
    expect(screen.queryByText("Add tags")).not.toBeInTheDocument();

    fireEvent.click(screen.getAllByText("Custom action")[0]!);

    await waitFor(() => {
      expect(onRun).toHaveBeenCalledWith(
        expect.objectContaining({
          selectedItemCount: 1,
          affectedAssetCount: 1,
          selectedAssetIds: ["asset-1"],
        }),
      );
    });
  });

  it("shows Add tags by default and opens the tag picker modal", async () => {
    render(
      <AssetsPageHeader
        sortBy="date_captured"
        onSortByChange={vi.fn()}
        title="Assets"
        browseItems={browseItems}
        hiddenBulkActions={[
          "set-rating",
          "set-liked",
          "stack-selected",
          "download",
          "delete-assets",
        ]}
      />,
    );

    const addTagsButtons = screen.getAllByText("Add tags");
    expect(addTagsButtons.length).toBeGreaterThan(0);
    fireEvent.click(addTagsButtons[0]!);

    expect(
      screen.getByText("Add tags to {{count}} selected items.", { exact: false }),
    ).toBeInTheDocument();
  });

  it("confirms custom actions that require confirmation", async () => {
    const onRun = vi.fn();

    render(
      <AssetsPageHeader
        sortBy="date_captured"
        onSortByChange={vi.fn()}
        title="Assets"
        browseItems={browseItems}
        hiddenBulkActions={[
          "set-rating",
          "set-liked",
          "stack-selected",
          "add-tags",
          "add-to-album",
          "download",
          "delete-assets",
        ]}
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

    fireEvent.click(screen.getAllByText("Confirm action")[0]!);

    expect(screen.getByText("Run confirmed action?")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Confirm"));

    await waitFor(() => {
      expect(onRun).toHaveBeenCalledOnce();
    });
  });
});
