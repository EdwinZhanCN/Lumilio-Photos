import { Image as ImageIcon } from "lucide-react";
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
import {
  pickAssetUserFilter,
  stripConstrainedAssetUserFilter,
  type AssetUserFilter,
  type AssetUserFilterKey,
} from "../model/filter";
import type { SortByType } from "../types";

const DEFAULT_LOCKED_FIELDS: readonly AssetUserFilterKey[] = ["type"];

type PhotoPickerContentProps = {
  onSelect: (id: string) => void;
  title?: string;
  initialFilters: AssetUserFilter;
  lockedFields: readonly AssetUserFilterKey[];
};

type PhotoPickerProps = {
  scopeId: string;
  onSelect: (id: string) => void;
  title?: string;
  initialFilters?: AssetUserFilter;
  lockedFields?: readonly AssetUserFilterKey[];
};

function PhotoPickerContent({
  onSelect,
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
    if (selection.enabled && selection.selectedCount > 0) {
      const id = resolveBrowseSelectedAssetIds(selection.selectedIds, browseItems)[0];
      if (id) {
        onSelect(id);
      }
    }
  }, [browseItems, selection.selectedIds, selection.selectedCount, selection.enabled, onSelect]);

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
  title,
  initialFilters,
  lockedFields = DEFAULT_LOCKED_FIELDS,
}: PhotoPickerProps): React.JSX.Element {
  const pickerInitialFilters = useMemo<AssetUserFilter>(
    () => ({
      ...initialFilters,
      type: "PHOTO",
    }),
    [initialFilters],
  );
  const pickerLockedFields = useMemo<readonly AssetUserFilterKey[]>(
    () => Array.from(new Set<AssetUserFilterKey>(["type", ...lockedFields])),
    [lockedFields],
  );

  return (
    <WorkerProvider preload={["justified"]}>
      <AssetBrowserScope
        scopeId={scopeId}
        defaultSelectionMode="single"
        initialSelection={{ selectionMode: "single" }}
      >
        <PhotoPickerContent
          onSelect={onSelect}
          title={title}
          initialFilters={pickerInitialFilters}
          lockedFields={pickerLockedFields}
        />
      </AssetBrowserScope>
    </WorkerProvider>
  );
}
