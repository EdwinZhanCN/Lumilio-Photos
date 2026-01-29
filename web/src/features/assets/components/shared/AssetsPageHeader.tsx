import PageHeader from "@/components/PageHeader";
import {
  PhotoIcon,
  VideoCameraIcon,
  MusicalNoteIcon,
} from "@heroicons/react/24/outline";
import { SquareMousePointer, FunnelIcon, Rocket, Trash2, FolderPlus, Heart, Star, Download, AlertTriangle, X, Plus } from "lucide-react";
import SearchBar from "@/components/SearchBar";
import FilterTool, {
  FilterDTO,
} from "@/features/assets/components/page/FilterTool/FilterTool";
import { useSelection, useBulkAssetOperations } from "@/features/assets/hooks/useSelection";
import { GroupByType } from "@/features/assets/types/assets.type";
import { selectTabTitle } from "@/features/assets/slices/ui.slice";
import { useCallback, useMemo, useRef, useEffect, useState, ReactNode } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import {
  useFilterState,
  useFilterActions,
  useCurrentTab,
  useSearchQuery,
} from "@/features/assets/selectors";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album, ApiResult, ListAlbumsResponse } from "@/lib/albums/types";

interface AssetsPageHeaderProps {
  groupBy: GroupByType;
  onGroupByChange: (groupBy: GroupByType) => void;
  onFiltersChange?: (filters: FilterDTO) => void;
  title?: string;
  icon?: ReactNode;
}

function mapFilenameModeToDTO(
  mode?: "contains" | "matches" | "startswith" | "endswith",
): "contains" | "matches" | "starts_with" | "ends_with" | undefined {
  switch (mode) {
    case "startswith":
      return "starts_with";
    case "endswith":
      return "ends_with";
    case "contains":
    case "matches":
      return mode;
    default:
      return undefined;
  }
}

function mapFilenameOperatorToMode(
  op?: "contains" | "matches" | "starts_with" | "ends_with",
): "contains" | "matches" | "startswith" | "endswith" | undefined {
  switch (op) {
    case "starts_with":
      return "startswith";
    case "ends_with":
      return "endswith";
    case "contains":
    case "matches":
      return op;
    default:
      return undefined;
  }
}


const AssetsPageHeader = ({
  groupBy,
  onGroupByChange,
  onFiltersChange,
  title,
  icon,
}: AssetsPageHeaderProps) => {

  const { t } = useI18n();
  const selection = useSelection();
  const bulkOps = useBulkAssetOperations();
  const showMessage = useMessage();

  // Selectors & Actions
  const filters = useFilterState();
  const { batchUpdateFilters } = useFilterActions();
  const currentTab = useCurrentTab();
  const searchQuery = useSearchQuery();
  // We use the prop callback for onGroupByChange, but we might want to dispatch too?
  // The prop onGroupByChange passed from parent (Photos.tsx) already calls setGroupBy action.
  // So we just use the prop. But for search effect we need to call it.

  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [isAlbumModalOpen, setIsAlbumModalOpen] = useState(false);
  const [albums, setAlbums] = useState<Album[]>([]);
  const [isLoadingAlbums, setIsLoadingAlbums] = useState(false);
  const [isAddingToAlbum, setIsAddingToAlbum] = useState(false);
  const listAlbumsMutation = $api.useMutation("get", "/api/v1/albums");

  // Hydrate FilterTool from global filters (single source of truth)
  const inboundDTO = useMemo(() => {
    const f = filters;
    if (!f?.enabled) return {};
    const dto: FilterDTO = {};
    if (typeof f.raw === "boolean") dto.raw = f.raw;
    if (typeof f.rating === "number") dto.rating = f.rating;
    if (typeof f.liked === "boolean") dto.liked = f.liked;
    if (f.filename && f.filename.value?.trim()) {
      dto.filename = {
        operator: mapFilenameModeToDTO(f.filename.mode) as any,
        value: f.filename.value,
      };
    }
    if (f.date && (f.date.from || f.date.to)) {
      dto.date = { from: f.date.from, to: f.date.to };
    }
    if (f.camera_make?.trim()) dto.camera_make = f.camera_make.trim();
    if (f.lens?.trim()) dto.lens = f.lens.trim();
    return dto;
  }, [filters]);

  const inboundHash = useMemo(
    () => JSON.stringify(inboundDTO || {}),
    [inboundDTO],
  );

  // Get tab-specific configuration
  const tabTitle = selectTabTitle(currentTab);

  // Get appropriate icon for current tab
  const TabIcon = useMemo(() => {
    switch (currentTab) {
      case "videos":
        return VideoCameraIcon;
      case "audios":
        return MusicalNoteIcon;
      default:
        return PhotoIcon;
    }
  }, [currentTab]);

  // Handle search activation (switch to flat view when searching)
  useEffect(() => {
    if (searchQuery.trim() && groupBy !== "flat") {
      onGroupByChangeRef.current("flat");
    }
  }, [searchQuery, groupBy]);

  // Use ref to store the latest onFiltersChange callback to avoid dependency issues
  const onFiltersChangeRef = useRef(onFiltersChange);
  const onGroupByChangeRef = useRef(onGroupByChange);

  useEffect(() => {
    onFiltersChangeRef.current = onFiltersChange;
    onGroupByChangeRef.current = onGroupByChange;
  });

  // Handle filter changes
  const handleFiltersChange = useCallback(
    (newFilters: FilterDTO) => {
      // Prevent re-emit loop when FilterTool mounts with initial values
      const nextHash = JSON.stringify(newFilters || {});
      if (nextHash === inboundHash) {
        onFiltersChangeRef.current?.(newFilters);
        return;
      }

      // Build a full payload resetting all fields first (single source of truth)
      const payload: any = {
        enabled: Object.keys(newFilters).length > 0,
        raw: undefined,
        rating: undefined,
        liked: undefined,
        filename: undefined,
        date: undefined,
        camera_make: undefined,
        lens: undefined,
      };

      if (newFilters.raw !== undefined) payload.raw = newFilters.raw;
      if (newFilters.rating !== undefined) payload.rating = newFilters.rating;
      if (newFilters.liked !== undefined) payload.liked = newFilters.liked;

      if (newFilters.filename && newFilters.filename.value.trim()) {
        payload.filename = {
          mode: mapFilenameOperatorToMode(newFilters.filename.operator),
          value: newFilters.filename.value.trim(),
        };
      }

      if (newFilters.date && (newFilters.date.from || newFilters.date.to)) {
        payload.date = {
          from: newFilters.date.from,
          to: newFilters.date.to,
        };
      }

      if (newFilters.camera_make && newFilters.camera_make.trim()) {
        payload.camera_make = newFilters.camera_make.trim();
      }

      if (newFilters.lens && newFilters.lens.trim()) {
        payload.lens = newFilters.lens.trim();
      }

      batchUpdateFilters(payload);

      // Switch to flat view when filtering for better result visibility
      if (payload.enabled && groupBy !== "flat") {
        onGroupByChangeRef.current("flat");
      }

      onFiltersChangeRef.current?.(newFilters);
    },
    [batchUpdateFilters, groupBy, inboundHash],
  );

  // Toggle selection mode
  const handleToggleSelection = useCallback(() => {
    selection.setEnabled(!selection.enabled);
  }, [selection]);

  const handleDeleteClick = () => {
    setIsDeleteConfirmOpen(true);
  };

  const confirmDelete = async () => {
    try {
      await bulkOps.bulkDelete();
      showMessage("success", t("assets.assetsPageHeader.messages.deleteSuccess", { count: selection.selectedCount }));
    } catch (error) {
      showMessage("error", t("assets.assetsPageHeader.messages.deleteError"));
    } finally {
      setIsDeleteConfirmOpen(false);
    }
  };

  const handleDownloadAll = async () => {
    try {
      await bulkOps.bulkDownload();
      showMessage("success", t("assets.assetsPageHeader.messages.downloadStart"));
    } catch (error) {
      showMessage("error", t("assets.assetsPageHeader.messages.downloadError"));
    }
  };

  const handleAddToAlbumClick = async () => {
    setIsAlbumModalOpen(true);
    setIsLoadingAlbums(true);
    try {
      const response = await listAlbumsMutation.mutateAsync({
        params: { query: { limit: 50 } },
      });
      const responseData = response as ApiResult<ListAlbumsResponse> | undefined;
      if (responseData?.data) {
        setAlbums(responseData.data.albums || []);
      }
    } catch (error) {
      showMessage("error", t("assets.assetsPageHeader.messages.loadAlbumsError"));
    } finally {
      setIsLoadingAlbums(false);
    }
  };

  const handleSelectAlbum = async (albumId: number) => {
    setIsAddingToAlbum(true);
    try {
      await bulkOps.bulkAddToAlbum(albumId);
      showMessage("success", t("assets.assetsPageHeader.messages.addToAlbumSuccess", { count: selection.selectedCount }));
      setIsAlbumModalOpen(false);
      selection.clear();
    } catch (error) {
      showMessage("error", t("assets.assetsPageHeader.messages.addToAlbumError"));
    } finally {
      setIsAddingToAlbum(false);
    }
  };

  return (
    <>
      <PageHeader
        title={title ?? tabTitle}
        icon={icon ?? <TabIcon className="w-6 h-6 text-primary" />}
      >
        {/* Group By Dropdown */}
        <div className="dropdown">
          <div
            tabIndex={0}
            role="button"
            className="btn btn-sm btn-soft btn-info"
          >
            <FunnelIcon className="size-4" />
            {t("assets.assetsPageHeader.groupBy", { groupBy })}
          </div>
          <ul
            tabIndex={0}
            className="dropdown-content menu bg-base-200 rounded-box z-[1] w-40 p-2 shadow"
          >
            <li>
              <a
                onClick={() => onGroupByChange("date")}
                className={groupBy === "date" ? "active" : ""}
              >
                {t("assets.assetsPageHeader.groupByOptions.date")}
              </a>
            </li>
            <li>
              <a
                onClick={() => onGroupByChange("type")}
                className={groupBy === "type" ? "active" : ""}
              >
                {t("assets.assetsPageHeader.groupByOptions.type")}
              </a>
            </li>
            <li>
              <a
                onClick={() => onGroupByChange("album")}
                className={groupBy === "album" ? "active" : ""}
              >
                {t("assets.assetsPageHeader.groupByOptions.album")}
              </a>
            </li>
            <li>
              <a
                onClick={() => onGroupByChange("flat")}
                className={groupBy === "flat" ? "active" : ""}
              >
                {t("assets.assetsPageHeader.groupByOptions.flat")}
              </a>
            </li>
          </ul>
        </div>

        {/* Filter Tool */}
        <FilterTool
          initial={inboundDTO}
          onChange={handleFiltersChange}
          autoApply={true}
        />

        {/* Selection Toggle Button */}
        <button
          className={`btn btn-sm btn-circle btn-soft btn-info ${selection.enabled ? "btn-active" : ""
            } relative`}
          onClick={handleToggleSelection}
          title={
            selection.enabled ? t("assets.assetsPageHeader.selectionMode.exit") : t("assets.assetsPageHeader.selectionMode.enter")
          }
        >
          <SquareMousePointer className="w-4 h-4" />
          {selection.selectedCount > 0 && (
            <span className="badge badge-xs badge-primary absolute -right-1 -top-1">
              {selection.selectedCount}
            </span>
          )}
        </button>
        {/* Quick Actions Rocket Menu - Only in selection mode */}
        {selection.enabled && (
          <div className="dropdown dropdown-end">
            <div
              tabIndex={0}
              role="button"
              className={`btn btn-sm btn-accent ${selection.selectedCount === 0 ? 'btn-disabled opacity-50' : ''}`}
              title={t("assets.assetsPageHeader.actions.title")}
            >
              <Rocket className="size-4" />
              {t("assets.assetsPageHeader.actions.title")}
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu bg-base-200 rounded-box z-[100] w-52 p-2 shadow-lg mt-2"
            >
              <li className="menu-title px-4 py-2 text-xs opacity-50 uppercase tracking-wider">
                {t("assets.assetsPageHeader.actions.applyToItems", { count: selection.selectedCount })}
              </li>
              <li>
                <button onClick={() => bulkOps.bulkToggleLike()}>
                  <Heart size={16} className="text-error" />
                  {t("assets.assetsPageHeader.actions.toggleLike")}
                </button>
              </li>
              <li>
                <div className="flex flex-col items-stretch p-0">
                  <div className="px-4 py-2 flex items-center gap-2 text-sm">
                    <Star size={16} className="text-warning" />
                    {t("assets.assetsPageHeader.actions.setRating")}
                  </div>
                  <div className="flex justify-around p-2 pt-0">
                    <button
                      className="btn btn-xs btn-ghost btn-square text-base-content/50"
                      onClick={() => bulkOps.bulkUpdateRating(0)}
                      title={t("assets.assetsPageHeader.actions.clearRating")}
                    >
                      <X size={12} />
                    </button>
                    {[1, 2, 3, 4, 5].map(r => (
                      <button
                        key={r}
                        className="btn btn-xs btn-ghost btn-square"
                        onClick={() => bulkOps.bulkUpdateRating(r)}
                      >
                        {r}
                      </button>
                    ))}
                  </div>
                </div>
              </li>
              <div className="divider my-1"></div>
              <li>
                <button className="text-info" onClick={handleAddToAlbumClick}>
                  <FolderPlus size={16} />
                  {t("assets.assetsPageHeader.actions.addToAlbum")}
                </button>
              </li>
              <li>
                <button onClick={handleDownloadAll}>
                  <Download size={16} />
                  {t("assets.assetsPageHeader.actions.downloadAll")}
                </button>
              </li>
              <li>
                <button className="text-error" onClick={handleDeleteClick}>
                  <Trash2 size={16} />
                  {t("assets.assetsPageHeader.actions.deleteSelected")}
                </button>
              </li>
            </ul>
          </div>
        )}

        {/* Search Bar */}
        <SearchBar enableSemanticSearch={currentTab === "photos"} />
      </PageHeader>

      {/* Delete Confirmation Modal */}
      {isDeleteConfirmOpen && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-error">
            <div className="flex items-center gap-3 text-error mb-4">
              <AlertTriangle size={24} />
              <h3 className="font-bold text-lg">{t("assets.assetsPageHeader.deleteConfirmModal.title")}</h3>
            </div>
            <p className="py-4">
              {t("assets.assetsPageHeader.deleteConfirmModal.message", { count: selection.selectedCount })}
            </p>
            <div className="modal-action">
              <button className="btn btn-ghost" onClick={() => setIsDeleteConfirmOpen(false)}>
                {t("assets.assetsPageHeader.deleteConfirmModal.cancelButton")}
              </button>
              <button className="btn btn-error gap-2" onClick={confirmDelete}>
                <Trash2 size={18} />
                {t("assets.assetsPageHeader.deleteConfirmModal.deleteButton")}
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={() => setIsDeleteConfirmOpen(false)}></div>
        </div>
      )}

      {/* Add to Album Modal */}
      {isAlbumModalOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-md">
            <div className="flex items-center justify-between mb-6">
              <h3 className="font-bold text-lg flex items-center gap-2">
                <FolderPlus className="text-primary" />
                {t("assets.assetsPageHeader.addToAlbumModal.title")}
              </h3>
              <button className="btn btn-sm btn-circle btn-ghost" onClick={() => setIsAlbumModalOpen(false)}>
                <X size={20} />
              </button>
            </div>

            <p className="text-sm opacity-70 mb-4">
              {t("assets.assetsPageHeader.addToAlbumModal.message", { count: selection.selectedCount })}
            </p>

            <div className="max-h-64 overflow-y-auto space-y-2 custom-scrollbar pr-1">
              {isLoadingAlbums ? (
                <div className="flex justify-center py-8">
                  <span className="loading loading-spinner loading-md text-primary"></span>
                  <span className="sr-only">{t("assets.assetsPageHeader.addToAlbumModal.loadingAlbums")}</span>
                </div>
              ) : albums.length > 0 ? (
                albums.map(album => (
                  <button
                    key={album.album_id}
                    className="btn btn-ghost btn-block justify-start gap-3 font-normal hover:bg-primary/10"
                    onClick={() => handleSelectAlbum(album.album_id!)}
                    disabled={isAddingToAlbum}
                  >
                    <div className="w-10 h-10 rounded bg-base-300 flex items-center justify-center overflow-hidden">
                      {album.cover_asset_id ? (
                        <img
                          src={assetUrls.getThumbnailUrl(album.cover_asset_id, "small")}
                          className="w-full h-full object-cover"
                          alt=""
                        />
                      ) : (
                        <FolderPlus size={18} className="opacity-30" />
                      )}
                    </div>
                    <div className="text-left">
                      <div className="font-bold text-sm">{album.album_name}</div>
                      <div className="text-[10px] opacity-50 uppercase tracking-wider">
                        {t("assets.assetsPageHeader.addToAlbumModal.itemCount", { count: album.asset_count || 0 })}
                      </div>
                    </div>
                  </button>
                ))
              ) : (
                <div className="text-center py-8 opacity-50">
                  <p>{t("assets.assetsPageHeader.addToAlbumModal.noAlbumsFound")}</p>
                </div>
              )}
            </div>

            <div className="modal-action border-t border-base-200 pt-4">
              <button className="btn btn-primary btn-sm gap-2">
                <Plus size={16} />
                {t("assets.assetsPageHeader.addToAlbumModal.createNewAlbum")}
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={() => setIsAlbumModalOpen(false)}></div>
        </div>
      )}
    </>
  );
};

export default AssetsPageHeader;
