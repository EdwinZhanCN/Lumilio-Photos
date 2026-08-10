import { Check, Image as ImageIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useI18n } from "@/lib/i18n";
import { AssetBrowserScope } from "../flows/browse/selection/AssetBrowserScope";
import SquareGallery from "../flows/browse/gallery/SquareGallery/SquareGallery";
import AssetsPageHeader from "../flows/browse/header/AssetsPageHeader";
import { useAssetBrowser } from "../flows/browse/useAssetBrowser";
import {
  useAssetSelection,
  useAssetSelectionActions,
} from "../flows/browse/selection/useAssetSelection";
import { resolveBrowseSelectedAssetIds } from "../model/browseItems";
import type { AssetsBulkActionInput } from "@/lib/assets/bulkActions";
import {
  pickAssetUserFilter,
  stripConstrainedAssetUserFilter,
  type AssetUserFilter,
  type AssetUserFilterKey,
} from "../model/filter";
import type { SortByType } from "../types";

const DEFAULT_LOCKED_FIELDS: readonly AssetUserFilterKey[] = ["type"];

type PhotoPickerContentProps = {
  onSelect?: (id: string) => void;
  onConfirm?: (ids: string[]) => Promise<void> | void;
  selectionMode: "single" | "multiple";
  confirmLabel?: string;
  isConfirming?: boolean;
  title?: string;
  initialFilters: AssetUserFilter;
  lockedFields: readonly AssetUserFilterKey[];
};

type PhotoPickerProps = {
  scopeId: string;
  onSelect?: (id: string) => void;
  onConfirm?: (ids: string[]) => Promise<void> | void;
  selectionMode?: "single" | "multiple";
  confirmLabel?: string;
  isConfirming?: boolean;
  /** Defaults to photos; pass null when the workflow accepts photos and videos. */
  typeFilter?: "PHOTO" | "VIDEO" | null;
  title?: string;
  initialFilters?: AssetUserFilter;
  lockedFields?: readonly AssetUserFilterKey[];
};

function PhotoPickerContent({
  onSelect,
  onConfirm,
  selectionMode,
  confirmLabel,
  isConfirming = false,
  title,
  initialFilters,
  lockedFields,
}: PhotoPickerContentProps): React.JSX.Element {
  const { t } = useI18n();
  const constraint = useMemo(
    () => pickAssetUserFilter(initialFilters, lockedFields),
    [initialFilters, lockedFields],
  );
  const [sortBy, setSortBy] = useState<SortByType>("date_captured");
  const [userFilter, setUserFilter] = useState<AssetUserFilter>(() =>
    stripConstrainedAssetUserFilter(initialFilters, constraint),
  );
  const { clear: clearSelection, setEnabled: setSelectionEnabled } = useAssetSelectionActions();
  const selection = useAssetSelection();

  const { browseGroups, browseItems, isLoading, isLoadingMore, fetchMore, hasMore, viewKey } =
    useAssetBrowser({
      withGroups: true,
      sortBy,
      constraint,
      userFilter,
    });

  const layoutKey = useMemo(() => {
    const itemIds = (browseItems ?? []).map((item) => item.id);
    return `${viewKey}:${itemIds.join(",")}`;
  }, [viewKey, browseItems]);

  useEffect(() => {
    clearSelection();
    setUserFilter(stripConstrainedAssetUserFilter(initialFilters, constraint));
    setSelectionEnabled(true);
  }, [clearSelection, constraint, initialFilters, setSelectionEnabled]);

  useEffect(() => {
    if (selectionMode === "single" && selection.enabled && selection.selectedCount > 0) {
      const id = resolveBrowseSelectedAssetIds(selection.selectedIds, browseItems)[0];
      if (id && onSelect) {
        onSelect(id);
      }
    }
  }, [
    browseItems,
    onSelect,
    selection.enabled,
    selection.selectedCount,
    selection.selectedIds,
    selectionMode,
  ]);

  const pickerBulkActions = useMemo<AssetsBulkActionInput | undefined>(() => {
    if (selectionMode !== "multiple" || !onConfirm) return undefined;
    return (context) => [
      {
        id: "confirm-photo-picker",
        label: confirmLabel ?? t("assets.photoPicker.confirm", "Add selected"),
        icon: isConfirming ? (
          <span className="loading loading-spinner loading-xs" />
        ) : (
          <Check className="size-4" />
        ),
        disabled: context.selectedAssetIds.length === 0 || isConfirming,
        onRun: async () => {
          await onConfirm(context.selectedAssetIds);
          context.clearSelection();
        },
      },
    ];
  }, [confirmLabel, isConfirming, onConfirm, selectionMode, t]);

  return (
    <div className="flex h-full flex-col overflow-hidden bg-base-100">
      <AssetsPageHeader
        sortBy={sortBy}
        onSortByChange={setSortBy}
        filter={userFilter}
        constraint={constraint}
        onFiltersChange={setUserFilter}
        browseItems={browseItems}
        title={
          title ??
          t("collections.createModal.coverPicker.title", {
            defaultValue: "Pick a photo",
          })
        }
        icon={<ImageIcon className="h-6 w-6 text-primary" />}
        bulkActions={pickerBulkActions}
        hiddenBulkActions={[
          "set-rating",
          "set-liked",
          "stack-selected",
          "add-tags",
          "add-to-album",
          "download",
          "delete-assets",
        ]}
      />
      <div className="custom-scrollbar flex-1 overflow-x-hidden overflow-y-auto">
        <SquareGallery
          browseGroups={browseGroups}
          key={layoutKey}
          openCarousel={() => {}}
          onLoadMore={fetchMore}
          hasMore={hasMore}
          isLoadingMore={isLoadingMore}
          isLoading={isLoading && browseItems.length === 0}
          columns={5}
          render3DCard={false}
        />
      </div>
    </div>
  );
}

export default function PhotoPicker({
  scopeId,
  onSelect,
  onConfirm,
  selectionMode = "single",
  confirmLabel,
  isConfirming = false,
  typeFilter = "PHOTO",
  title,
  initialFilters,
  lockedFields = DEFAULT_LOCKED_FIELDS,
}: PhotoPickerProps): React.JSX.Element {
  const pickerInitialFilters = useMemo<AssetUserFilter>(
    () => ({
      ...initialFilters,
      type: typeFilter ?? undefined,
    }),
    [initialFilters, typeFilter],
  );
  const pickerLockedFields = useMemo<readonly AssetUserFilterKey[]>(
    () =>
      typeFilter
        ? Array.from(new Set<AssetUserFilterKey>(["type", ...lockedFields]))
        : lockedFields.filter((field) => field !== "type"),
    [lockedFields, typeFilter],
  );

  return (
    <WorkerProvider preload={["justified"]}>
      <AssetBrowserScope
        scopeId={scopeId}
        defaultSelectionMode={selectionMode}
        initialSelection={{ selectionMode }}
      >
        <PhotoPickerContent
          onSelect={onSelect}
          onConfirm={onConfirm}
          selectionMode={selectionMode}
          confirmLabel={confirmLabel}
          isConfirming={isConfirming}
          title={title}
          initialFilters={pickerInitialFilters}
          lockedFields={pickerLockedFields}
        />
      </AssetBrowserScope>
    </WorkerProvider>
  );
}
