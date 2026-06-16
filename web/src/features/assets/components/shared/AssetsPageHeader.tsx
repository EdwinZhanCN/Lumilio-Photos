import PageHeader from "@/components/PageHeader";
import {
  SquareMousePointer,
  FunnelIcon,
  Rocket,
  Trash2,
  FolderPlus,
  Heart,
  Download,
  AlertTriangle,
  X,
  Plus,
  RefreshCcwDot,
  Ellipsis,
  ArrowUpDown,
  Star,
  ImageIcon,
} from "lucide-react";
import FilterTool, {
  FilterDTO,
  type FilterFieldKey,
} from "@/features/assets/components/page/FilterTool/FilterTool";
import {
  mapFilenameModeToDTO,
  mapFilenameOperatorToMode,
} from "@/features/assets/utils/filterUtils";
import {
  useSelection,
  useBulkAssetOperations,
} from "@/features/assets/hooks/useSelection";
import { BrowseItem, SortByType } from "@/features/assets/types/assets.type";
import {
  useCallback,
  useMemo,
  useRef,
  useEffect,
  useState,
  ReactNode,
} from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { useFilterState, useFilterActions } from "@/features/assets/selectors";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album, ApiResult, ListAlbumsResponse } from "@/lib/albums/types";
import { useWorkingRepository } from "@/features/settings";
import { useRepositoryScan } from "@/features/manage/hooks/useRepositoryScan";
import {
  getBrowseItemAsset,
  resolveBrowseSelectedAssetIds,
  resolveSelectedBrowseItems,
} from "@/features/assets/utils/browseItems";

type ConfirmableBulkAction =
  | { type: "rating"; rating: number }
  | { type: "liked"; liked: boolean };

interface AssetsPageHeaderProps {
  sortBy: SortByType;
  onSortByChange: (sortBy: SortByType) => void;
  onFiltersChange?: (filters: FilterDTO) => void;
  title?: string;
  icon?: ReactNode;
  browseItems?: BrowseItem[];
  lockedFilterFields?: readonly FilterFieldKey[];
}

const AssetsPageHeader = ({
  sortBy,
  onSortByChange,
  onFiltersChange,
  title,
  icon,
  browseItems,
  lockedFilterFields,
}: AssetsPageHeaderProps) => {
  const { t } = useI18n();
  const selection = useSelection();
  const showMessage = useMessage();

  const activeSortByLabel = useMemo(() => {
    switch (sortBy) {
      case "recently_added":
        return t("assets.assetsPageHeader.sortByOptions.recentlyAdded");
      default:
        return t("assets.assetsPageHeader.sortByOptions.dateCaptured");
    }
  }, [sortBy, t]);

  // Selectors & Actions
  const filters = useFilterState();
  const { batchUpdateFilters } = useFilterActions();

  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [confirmableBulkAction, setConfirmableBulkAction] =
    useState<ConfirmableBulkAction | null>(null);
  const [isAlbumModalOpen, setIsAlbumModalOpen] = useState(false);
  const [albums, setAlbums] = useState<Album[]>([]);
  const [isLoadingAlbums, setIsLoadingAlbums] = useState(false);
  const [isAddingToAlbum, setIsAddingToAlbum] = useState(false);
  const listAlbumsMutation = $api.useMutation("get", "/api/v1/albums");
  const { repositories, selectedRepository, scopeLabel } =
    useWorkingRepository();
  const { scanRepositories, isScanning } = useRepositoryScan();

  // Hydrate FilterTool from global filters (single source of truth)
  const inboundDTO = useMemo(() => {
    const f = filters;
    if (!f?.enabled) return {};
    const dto: FilterDTO = {};
    if (f.type === "PHOTO" || f.type === "VIDEO") dto.type = f.type;
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
    if (f.camera_model?.trim()) dto.camera_model = f.camera_model.trim();
    if (f.lens?.trim()) dto.lens = f.lens.trim();
    if (f.location) dto.location = { ...f.location };
    return dto;
  }, [filters]);

  const inboundHash = useMemo(
    () => JSON.stringify(inboundDTO || {}),
    [inboundDTO],
  );

  const tabTitle = useMemo(() => {
    return t("assets.all.title");
  }, [t]);

  // Use ref to store the latest onFiltersChange callback to avoid dependency issues
  const onFiltersChangeRef = useRef(onFiltersChange);

  useEffect(() => {
    onFiltersChangeRef.current = onFiltersChange;
  });

  // Handle filter changes
  const handleFiltersChange = useCallback(
    (newFilters: FilterDTO) => {
      // Prevent re-emit loop when FilterTool mounts with initial values
      const nextHash = JSON.stringify(newFilters || {});
      if (nextHash === inboundHash) {
        return;
      }

      // Build a full payload resetting all fields first (single source of truth)
      const payload: any = {
        enabled: Object.keys(newFilters).length > 0,
        type: undefined,
        raw: undefined,
        rating: undefined,
        liked: undefined,
        filename: undefined,
        date: undefined,
        camera_model: undefined,
        lens: undefined,
        location: undefined,
      };

      if (newFilters.type === "PHOTO" || newFilters.type === "VIDEO") {
        payload.type = newFilters.type;
      }
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

      if (newFilters.camera_model && newFilters.camera_model.trim()) {
        payload.camera_model = newFilters.camera_model.trim();
      }

      if (newFilters.lens && newFilters.lens.trim()) {
        payload.lens = newFilters.lens.trim();
      }

      if (newFilters.location) {
        payload.location = { ...newFilters.location };
      }

      batchUpdateFilters(payload);

      onFiltersChangeRef.current?.(newFilters);
    },
    [batchUpdateFilters, inboundHash],
  );

  // Toggle selection mode
  const handleToggleSelection = useCallback(() => {
    selection.setEnabled(!selection.enabled);
  }, [selection]);

  const handleScanCurrentLibrary = useCallback(async () => {
    const targetRepositoryIds = selectedRepository
      ? [selectedRepository.id]
      : repositories.map((repository) => repository.id).filter(Boolean);

    if (targetRepositoryIds.length === 0) {
      showMessage("info", t("assets.assetsPageHeader.scan.noRepositories"));
      return;
    }

    try {
      await scanRepositories(targetRepositoryIds);
      showMessage(
        "success",
        selectedRepository
          ? t("assets.assetsPageHeader.scan.currentQueued", {
              name: scopeLabel,
            })
          : t("assets.assetsPageHeader.scan.allQueued", {
              count: targetRepositoryIds.length,
            }),
      );
    } catch (error) {
      showMessage(
        "error",
        error instanceof Error
          ? error.message
          : t("assets.assetsPageHeader.scan.failed"),
      );
    }
  }, [
    repositories,
    scanRepositories,
    scopeLabel,
    selectedRepository,
    showMessage,
    t,
  ]);

  const handleDeleteClick = () => {
    setIsDeleteConfirmOpen(true);
  };

  const confirmDelete = async () => {
    try {
      await bulkOps.bulkDelete();
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.deleteSuccess", {
          count: affectedAssetCount,
        }),
      );
    } catch {
      showMessage("error", t("assets.assetsPageHeader.messages.deleteError"));
    } finally {
      setIsDeleteConfirmOpen(false);
    }
  };

  const effectiveBrowseItems = browseItems ?? [];

  const selectedBrowseItems = useMemo(() => {
    if (!effectiveBrowseItems || effectiveBrowseItems.length === 0) return [];
    return resolveSelectedBrowseItems(
      selection.selectedIds,
      effectiveBrowseItems,
    );
  }, [effectiveBrowseItems, selection.selectedIds]);

  const resolvedSelectedAssetIds = useMemo(
    () =>
      resolveBrowseSelectedAssetIds(
        selection.selectedIds,
        effectiveBrowseItems,
        {
          stackMode: "whole-stack",
        },
      ),
    [effectiveBrowseItems, selection.selectedIds],
  );

  const bulkOps = useBulkAssetOperations(resolvedSelectedAssetIds);
  const affectedAssetCount = resolvedSelectedAssetIds.length;
  const selectedItemCount =
    selectedBrowseItems.length || selection.selectedCount;
  const showAffectedAssetCount =
    affectedAssetCount > 0 && affectedAssetCount !== selectedItemCount;

  // Compute selected assets for operations that need the object (e.g. download filename)
  // We use useMemo to avoid re-calculation on every render
  const selectedAssets = useMemo(() => {
    if (!selection.enabled || selection.selectedCount === 0) return [];
    return selectedBrowseItems.flatMap((item) =>
      item.type === "stack" ? item.assets : [getBrowseItemAsset(item)],
    );
  }, [selection.enabled, selection.selectedCount, selectedBrowseItems]);

  const handleDownloadAll = async () => {
    try {
      await bulkOps.bulkDownload(selectedAssets);
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.downloadStart"),
      );
    } catch {
      showMessage("error", t("assets.assetsPageHeader.messages.downloadError"));
    }
  };

  const confirmBulkAction = async () => {
    if (!confirmableBulkAction) return;

    try {
      if (confirmableBulkAction.type === "rating") {
        await bulkOps.bulkUpdateRating(confirmableBulkAction.rating);
      } else {
        await bulkOps.bulkSetLike(confirmableBulkAction.liked);
      }
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.bulkActionSuccess", {
          count: affectedAssetCount,
          defaultValue: "Updated {{count}} assets.",
        }),
      );
    } catch {
      showMessage(
        "error",
        t("assets.assetsPageHeader.messages.bulkActionError", {
          defaultValue: "Failed to update selected assets.",
        }),
      );
    } finally {
      setConfirmableBulkAction(null);
    }
  };

  const handleAddToAlbumClick = async () => {
    setIsAlbumModalOpen(true);
    setIsLoadingAlbums(true);
    try {
      const response = await listAlbumsMutation.mutateAsync({
        params: { query: { limit: 50 } },
      });
      const responseData = response as
        | ApiResult<ListAlbumsResponse>
        | undefined;
      if (responseData?.data) {
        setAlbums(responseData.data.albums || []);
      }
    } catch {
      showMessage(
        "error",
        t("assets.assetsPageHeader.messages.loadAlbumsError"),
      );
    } finally {
      setIsLoadingAlbums(false);
    }
  };

  const handleSelectAlbum = async (albumId: number) => {
    setIsAddingToAlbum(true);
    try {
      await bulkOps.bulkAddToAlbum(albumId);
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.addToAlbumSuccess", {
          count: affectedAssetCount,
        }),
      );
      setIsAlbumModalOpen(false);
      selection.clear();
    } catch {
      showMessage(
        "error",
        t("assets.assetsPageHeader.messages.addToAlbumError"),
      );
    } finally {
      setIsAddingToAlbum(false);
    }
  };

  // Close the parent dropdown menu when a menu item is clicked
  const handleDropdownItemClick = () => {
    const elem = document.activeElement;
    if (elem instanceof HTMLElement) {
      elem.blur();
    }
  };

  const ratingOptions = [
    { rating: 5, label: "★★★★★", valueLabel: "5" },
    { rating: 4, label: "★★★★", valueLabel: "4" },
    { rating: 3, label: "★★★", valueLabel: "3" },
    { rating: 2, label: "★★", valueLabel: "2" },
    { rating: 1, label: "★", valueLabel: "1" },
    {
      rating: 0,
      label: t("assets.assetsPageHeader.actions.unrated", {
        defaultValue: "Unrated",
      }),
      valueLabel: "0",
    },
  ];

  const renderAffectedAssetHint = () =>
    showAffectedAssetCount
      ? t("assets.assetsPageHeader.actions.affectsAssets", {
          selectedCount: selectedItemCount,
          assetCount: affectedAssetCount,
          defaultValue:
            "{{selectedCount}} selected items will affect {{assetCount}} assets.",
        })
      : t("assets.assetsPageHeader.actions.affectsSelected", {
          count: selectedItemCount,
          defaultValue: "{{count}} selected items.",
        });

  const renderBulkActionItems = () => (
    <>
      <li>
        <details>
          <summary>
            <Star size={16} />
            {t("assets.assetsPageHeader.actions.setRating", {
              defaultValue: "Set Rating",
            })}
          </summary>
          <ul>
            {ratingOptions.map((option) => (
              <li key={option.rating}>
                <button
                  type="button"
                  onClick={() => {
                    setConfirmableBulkAction({
                      type: "rating",
                      rating: option.rating,
                    });
                    handleDropdownItemClick();
                  }}
                >
                  <span className="min-w-20">{option.label}</span>
                  <span className="ml-auto opacity-50">
                    {option.valueLabel}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </details>
      </li>
      <li>
        <details>
          <summary>
            <Heart size={16} />
            {t("assets.assetsPageHeader.actions.likedMenu", {
              defaultValue: "Liked",
            })}
          </summary>
          <ul>
            <li>
              <button
                type="button"
                onClick={() => {
                  setConfirmableBulkAction({ type: "liked", liked: true });
                  handleDropdownItemClick();
                }}
              >
                {t("assets.assetsPageHeader.actions.like", {
                  defaultValue: "Liked",
                })}
              </button>
            </li>
            <li>
              <button
                type="button"
                onClick={() => {
                  setConfirmableBulkAction({ type: "liked", liked: false });
                  handleDropdownItemClick();
                }}
              >
                {t("assets.assetsPageHeader.actions.unlike", {
                  defaultValue: "Unliked",
                })}
              </button>
            </li>
          </ul>
        </details>
      </li>
    </>
  );

  return (
    <>
      <PageHeader
        title={title ?? tabTitle}
        icon={icon ?? <ImageIcon className="w-6 h-6 text-primary" />}
        className="sticky top-0 z-40 bg-base-100 border-b border-base-200"
      >
        {selection.enabled && (
          <div className="badge badge-lg badge-neutral hidden gap-2 rounded-full px-3 py-3 text-xs font-medium sm:inline-flex shrink-0">
            {t("assets.assetsPageHeader.selectionMode.selectedCount", {
              count: selection.selectedCount,
              defaultValue: "{{count}} selected",
            })}
          </div>
        )}

        {/*
          FULL MODE ACTIONS
          Visible on larger screens when search is NOT active
        */}
        <div className="items-center gap-2 hidden lg:flex">
          {/* Sort By Dropdown */}
          <div className="dropdown">
            <div
              tabIndex={0}
              role="button"
              className="btn btn-sm btn-soft btn-info rounded-full"
            >
              <FunnelIcon className="size-4" />
              {t("assets.assetsPageHeader.sortBy", {
                sortBy: activeSortByLabel,
              })}
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu bg-base-200 rounded-box z-[100] w-40 p-2 shadow-xl"
            >
              <li>
                <button
                  onClick={() => onSortByChange("date_captured")}
                  className={sortBy === "date_captured" ? "active" : ""}
                >
                  {t("assets.assetsPageHeader.sortByOptions.dateCaptured")}
                </button>
              </li>
              <li>
                <button
                  onClick={() => onSortByChange("recently_added")}
                  className={sortBy === "recently_added" ? "active" : ""}
                >
                  {t("assets.assetsPageHeader.sortByOptions.recentlyAdded")}
                </button>
              </li>
            </ul>
          </div>

          <FilterTool
            initial={inboundDTO}
            onChange={handleFiltersChange}
            autoApply={true}
            lockedFields={lockedFilterFields}
          />

          <button
            type="button"
            className="btn btn-sm btn-soft btn-info gap-2 rounded-full"
            onClick={handleScanCurrentLibrary}
            disabled={isScanning || repositories.length === 0}
            title={t("assets.assetsPageHeader.scan.title", {
              scope: scopeLabel,
            })}
          >
            {isScanning ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <RefreshCcwDot className="h-4 w-4" />
            )}
            <span className="hidden xl:inline text-xs font-medium">
              {t("assets.assetsPageHeader.scan.label")}
            </span>
          </button>

          <button
            type="button"
            className={`btn btn-sm btn-soft btn-info gap-2 rounded-full ${
              selection.enabled ? "btn-active" : ""
            }`}
            onClick={handleToggleSelection}
            title={
              selection.enabled
                ? t("assets.assetsPageHeader.selectionMode.exit")
                : t("assets.assetsPageHeader.selectionMode.enter")
            }
          >
            <SquareMousePointer className="w-4 h-4" />
            <span className="hidden xl:inline text-xs font-medium">
              {t("assets.assetsPageHeader.selectionMode.label", {
                defaultValue: "Select",
              })}
            </span>
          </button>

          {/* Quick Actions Rocket Menu - Only in selection mode */}
          {selection.enabled && (
            <div className="dropdown dropdown-end">
              <div
                tabIndex={0}
                role="button"
                className={`btn btn-sm btn-soft btn-accent gap-2 rounded-full ${
                  selection.selectedCount === 0 ? "btn-disabled opacity-50" : ""
                }`}
                title={t("assets.assetsPageHeader.actions.title")}
              >
                <Rocket className="size-4" />
                <span className="hidden xl:inline">
                  {t("assets.assetsPageHeader.actions.title")}
                </span>
                <span className="rounded-full bg-base-100/90 px-2.5 py-1 text-[11px] font-semibold text-base-content/70">
                  {selection.selectedCount}
                </span>
              </div>
              <ul
                tabIndex={0}
                className="dropdown-content menu z-[100] mt-2 w-64 rounded-box bg-base-100 p-3 shadow-xl border border-base-200"
              >
                <li className="menu-title px-3 py-2 text-xs uppercase tracking-[0.18em] text-base-content/45">
                  {t("assets.assetsPageHeader.actions.applyToItems", {
                    count: selection.selectedCount,
                  })}
                </li>
                {renderBulkActionItems()}
                <div className="divider my-1"></div>
                <li>
                  <button
                    type="button"
                    className="text-info"
                    onClick={() => {
                      void handleAddToAlbumClick();
                      handleDropdownItemClick();
                    }}
                  >
                    <FolderPlus size={16} />
                    {t("assets.assetsPageHeader.actions.addToAlbum")}
                  </button>
                </li>
                <li>
                  <button
                    type="button"
                    onClick={() => {
                      void handleDownloadAll();
                      handleDropdownItemClick();
                    }}
                  >
                    <Download size={16} />
                    {t("assets.assetsPageHeader.actions.downloadAll")}
                  </button>
                </li>
                <li>
                  <button
                    type="button"
                    className="text-error"
                    onClick={() => {
                      handleDeleteClick();
                      handleDropdownItemClick();
                    }}
                  >
                    <Trash2 size={16} />
                    {t("assets.assetsPageHeader.actions.deleteSelected")}
                  </button>
                </li>
              </ul>
            </div>
          )}
        </div>

        {/*
          COMPACT MODE ACTIONS MENU
          Visible on small screens
        */}
        <div className="dropdown dropdown-end m-0 shrink-0 lg:hidden block">
          <div
            tabIndex={0}
            role="button"
            className="btn btn-sm btn-soft btn-circle btn-info m-0"
            title={t("assets.assetsPageHeader.moreActions")}
          >
            <Ellipsis className="w-4 h-4" />
          </div>
          <ul
            tabIndex={0}
            className="dropdown-content menu bg-base-100 rounded-box z-[100] w-56 p-2 shadow-xl border border-base-200 mt-2"
          >
            <li className="menu-title text-xs text-base-content/45">
              {t("assets.assetsPageHeader.compactMenu.viewSort", {
                defaultValue: "View & Sort",
              })}
            </li>
            <li>
              <details open>
                <summary className="py-2">
                  <ArrowUpDown size={16} />
                  {t("assets.assetsPageHeader.sortBy", { sortBy: "" })}
                </summary>
                <ul>
                  <li>
                    <button
                      onClick={() => {
                        onSortByChange("date_captured");
                        handleDropdownItemClick();
                      }}
                      className={sortBy === "date_captured" ? "active" : ""}
                    >
                      {t("assets.assetsPageHeader.sortByOptions.dateCaptured")}
                    </button>
                  </li>
                  <li>
                    <button
                      onClick={() => {
                        onSortByChange("recently_added");
                        handleDropdownItemClick();
                      }}
                      className={sortBy === "recently_added" ? "active" : ""}
                    >
                      {t("assets.assetsPageHeader.sortByOptions.recentlyAdded")}
                    </button>
                  </li>
                </ul>
              </details>
            </li>

            <li className="menu-title mt-2 text-xs text-base-content/45">
              {t("assets.assetsPageHeader.compactMenu.actions", {
                defaultValue: "Actions",
              })}
            </li>
            <li>
              <button
                onClick={() => {
                  handleToggleSelection();
                  handleDropdownItemClick();
                }}
                className={selection.enabled ? "text-primary font-medium" : ""}
              >
                <SquareMousePointer size={16} />
                {selection.enabled
                  ? t("assets.assetsPageHeader.selectionMode.exit")
                  : t("assets.assetsPageHeader.selectionMode.enter")}
              </button>
            </li>
            <li>
              <button
                onClick={() => {
                  void handleScanCurrentLibrary();
                  handleDropdownItemClick();
                }}
                disabled={isScanning || repositories.length === 0}
              >
                {isScanning ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <RefreshCcwDot size={16} />
                )}
                {t("assets.assetsPageHeader.scan.label")}
              </button>
            </li>

            {selection.enabled && (
              <>
                <li className="menu-title mt-2 text-xs text-base-content/45">
                  <span>
                    {t("assets.assetsPageHeader.compactMenu.selectedItems", {
                      defaultValue: "Selected Items",
                    })}{" "}
                    ({selection.selectedCount})
                  </span>
                </li>
                <li>
                  <details>
                    <summary className="py-2 text-accent">
                      <Rocket size={16} />
                      {t("assets.assetsPageHeader.actions.title")}
                    </summary>
                    <ul className="w-full">
                      {renderBulkActionItems()}
                      <li>
                        <button
                          onClick={() => {
                            void handleAddToAlbumClick();
                            handleDropdownItemClick();
                          }}
                        >
                          <FolderPlus size={16} className="text-info" />{" "}
                          {t("assets.assetsPageHeader.actions.addToAlbum")}
                        </button>
                      </li>
                      <li>
                        <button
                          onClick={() => {
                            void handleDownloadAll();
                            handleDropdownItemClick();
                          }}
                        >
                          <Download size={16} />{" "}
                          {t("assets.assetsPageHeader.actions.downloadAll")}
                        </button>
                      </li>
                      <li>
                        <button
                          onClick={() => {
                            handleDeleteClick();
                            handleDropdownItemClick();
                          }}
                          className="text-error focus:bg-error/20"
                        >
                          <Trash2 size={16} />{" "}
                          {t("assets.assetsPageHeader.actions.deleteSelected")}
                        </button>
                      </li>
                    </ul>
                  </details>
                </li>
              </>
            )}
          </ul>
        </div>

        {/* Filter Tool in Compact Mode (placed outside Ellipsis menu because it's a complex component) */}
        <div className="shrink-0 ml-2 lg:hidden block">
          <FilterTool
            initial={inboundDTO}
            onChange={handleFiltersChange}
            autoApply={true}
            lockedFields={lockedFilterFields}
          />
        </div>
      </PageHeader>

      {/* Bulk Action Confirmation Modal */}
      {confirmableBulkAction && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-primary">
            <div className="mb-4 flex items-center gap-3 text-primary">
              {confirmableBulkAction.type === "rating" ? (
                <Star size={24} />
              ) : (
                <Heart size={24} />
              )}
              <h3 className="text-lg font-bold">
                {confirmableBulkAction.type === "rating"
                  ? t("assets.assetsPageHeader.bulkConfirm.ratingTitle", {
                      rating: confirmableBulkAction.rating,
                      defaultValue:
                        confirmableBulkAction.rating === 0
                          ? "Set selected assets as unrated?"
                          : "Set selected assets to {{rating}} stars?",
                    })
                  : t("assets.assetsPageHeader.bulkConfirm.likedTitle", {
                      action: confirmableBulkAction.liked
                        ? t("assets.assetsPageHeader.actions.like", {
                            defaultValue: "Liked",
                          })
                        : t("assets.assetsPageHeader.actions.unlike", {
                            defaultValue: "Unliked",
                          }),
                      defaultValue: "Update liked status?",
                    })}
              </h3>
            </div>
            <p className="py-4 text-sm text-base-content/70">
              {renderAffectedAssetHint()}
            </p>
            <div className="modal-action">
              <button
                className="btn btn-ghost"
                onClick={() => setConfirmableBulkAction(null)}
              >
                {t("common.cancel")}
              </button>
              <button className="btn btn-primary" onClick={confirmBulkAction}>
                {t("common.confirm", { defaultValue: "Confirm" })}
              </button>
            </div>
          </div>
          <div
            className="modal-backdrop"
            onClick={() => setConfirmableBulkAction(null)}
          ></div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {isDeleteConfirmOpen && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-error">
            <div className="flex items-center gap-3 text-error mb-4">
              <AlertTriangle size={24} />
              <h3 className="font-bold text-lg">
                {t("assets.assetsPageHeader.deleteConfirmModal.title")}
              </h3>
            </div>
            <p className="py-4">
              {t("assets.assetsPageHeader.deleteConfirmModal.message", {
                count: selectedItemCount,
              })}
              {showAffectedAssetCount && (
                <span className="mt-2 block text-sm text-base-content/60">
                  {renderAffectedAssetHint()}
                </span>
              )}
            </p>
            <div className="modal-action">
              <button
                className="btn btn-ghost"
                onClick={() => setIsDeleteConfirmOpen(false)}
              >
                {t("assets.assetsPageHeader.deleteConfirmModal.cancelButton")}
              </button>
              <button className="btn btn-error gap-2" onClick={confirmDelete}>
                <Trash2 size={18} />
                {t("assets.assetsPageHeader.deleteConfirmModal.deleteButton")}
              </button>
            </div>
          </div>
          <div
            className="modal-backdrop"
            onClick={() => setIsDeleteConfirmOpen(false)}
          ></div>
        </div>
      )}

      {/* Add to Album Modal */}
      {isAlbumModalOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-md h-[80vh] flex flex-col p-0 overflow-hidden">
            {/* Header */}
            <div className="flex items-center justify-between px-5 py-3 border-b border-base-200 shrink-0">
              <h3 className="font-bold text-lg flex items-center gap-2">
                <FolderPlus className="text-primary" size={20} />
                {t("assets.assetsPageHeader.addToAlbumModal.title")}
              </h3>
              <button
                className="btn btn-sm btn-circle btn-ghost"
                onClick={() => setIsAlbumModalOpen(false)}
              >
                <X size={20} />
              </button>
            </div>

            {/* Hint */}
            <p className="text-sm opacity-70 px-5 py-2 shrink-0">
              {t("assets.assetsPageHeader.addToAlbumModal.message", {
                count: selectedItemCount,
              })}
              {showAffectedAssetCount && (
                <span className="mt-1 block">{renderAffectedAssetHint()}</span>
              )}
            </p>

            {/* Scrollable List */}
            <div className="flex-1 min-h-0 overflow-y-auto custom-scrollbar px-3">
              {isLoadingAlbums ? (
                <div className="flex justify-center py-12">
                  <span className="loading loading-spinner loading-md text-primary"></span>
                </div>
              ) : albums.length > 0 ? (
                <ul className="list bg-base-200/50 rounded-box">
                  <li className="p-4 pb-2 text-xs opacity-60 tracking-wide">
                    {t("assets.assetsPageHeader.addToAlbumModal.itemCount", {
                      count: albums.length,
                    })}
                  </li>
                  {albums.map((album) => (
                    <li key={album.album_id} className="list-row">
                      <div className="size-10 rounded-box overflow-hidden bg-base-300 flex-shrink-0">
                        {album.cover_asset_id ? (
                          <img
                            src={assetUrls.getThumbnailUrl(
                              album.cover_asset_id,
                              "small",
                            )}
                            className="size-full object-cover"
                            alt=""
                          />
                        ) : (
                          <div className="size-full flex items-center justify-center opacity-30">
                            <FolderPlus size={18} />
                          </div>
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="font-semibold text-sm truncate">
                          {album.album_name}
                        </div>
                        <div className="text-xs opacity-50">
                          {t(
                            "assets.assetsPageHeader.addToAlbumModal.itemCount",
                            { count: album.asset_count || 0 },
                          )}
                        </div>
                      </div>
                      <button
                        className="btn btn-sm btn-ghost btn-square"
                        onClick={() => handleSelectAlbum(album.album_id!)}
                        disabled={isAddingToAlbum}
                      >
                        <Plus size={18} />
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <div className="text-center py-12 opacity-50">
                  <p>
                    {t("assets.assetsPageHeader.addToAlbumModal.noAlbumsFound")}
                  </p>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="border-t border-base-200 px-5 py-3 shrink-0">
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => setIsAlbumModalOpen(false)}
              >
                {t("common.cancel")}
              </button>
            </div>
          </div>
          <div
            className="modal-backdrop"
            onClick={() => setIsAlbumModalOpen(false)}
          ></div>
        </div>
      )}
    </>
  );
};

export default AssetsPageHeader;
