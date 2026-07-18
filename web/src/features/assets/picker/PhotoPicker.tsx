import { Image as ImageIcon } from "lucide-react";
import { useEffect, useMemo } from "react";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useI18n } from "@/lib/i18n";
import { AssetsProvider } from "../state/AssetsProvider";
import SquareGallery from "../components/browse/SquareGallery/SquareGallery";
import type { FilterDTO, FilterFieldKey } from "../components/browse/FilterTool/FilterTool";
import AssetsPageHeader from "../components/browse/AssetsPageHeader";
import { useCurrentAssetsView } from "../api/useAssetsView";
import { useSelection } from "../hooks/useSelection";
import { useFilterActions, useSelectionActions, useSortBy, useUIActions } from "../state/selectors";
import { resolveBrowseSelectedAssetIds } from "../utils/browseItems";

const DEFAULT_LOCKED_FIELDS: readonly FilterFieldKey[] = ["type"];

type PhotoPickerContentProps = {
  onSelect: (id: string) => void;
  title?: string;
  initialFilters: FilterDTO;
  lockedFields: readonly FilterFieldKey[];
};

type PhotoPickerProps = {
  scopeId: string;
  onSelect: (id: string) => void;
  title?: string;
  initialFilters?: FilterDTO;
  lockedFields?: readonly FilterFieldKey[];
};

function PhotoPickerContent({
  onSelect,
  title,
  initialFilters,
  lockedFields,
}: PhotoPickerContentProps): React.JSX.Element {
  const { t } = useI18n();
  const sortBy = useSortBy();
  const { setSortBy, setSearchQuery } = useUIActions();
  const { resetFilters, batchUpdateFilters } = useFilterActions();
  const { clear: clearSelection, setEnabled: setSelectionEnabled } = useSelectionActions();
  const selection = useSelection();

  const { browseGroups, browseItems, isLoading, isLoadingMore, fetchMore, hasMore, viewKey } =
    useCurrentAssetsView({
      withGroups: true,
      sortBy,
    });

  const layoutKey = useMemo(() => {
    const itemIds = (browseItems ?? []).map((item) => item.id);
    return `${viewKey}:${itemIds.join(",")}`;
  }, [viewKey, browseItems]);

  useEffect(() => {
    clearSelection();
    resetFilters();
    batchUpdateFilters({
      enabled: true,
      type: "PHOTO",
      raw: initialFilters.raw,
    });
    setSearchQuery("");
    setSelectionEnabled(true);
  }, [
    batchUpdateFilters,
    clearSelection,
    initialFilters.raw,
    resetFilters,
    setSearchQuery,
    setSelectionEnabled,
  ]);

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
        browseItems={browseItems}
        title={
          title ??
          t("collections.createModal.coverPicker.title", {
            defaultValue: "Pick a photo",
          })
        }
        icon={<ImageIcon className="h-6 w-6 text-primary" />}
        lockedFilterFields={lockedFields}
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
  const pickerInitialFilters = useMemo<FilterDTO>(
    () => ({
      ...initialFilters,
      type: "PHOTO",
    }),
    [initialFilters],
  );
  const pickerLockedFields = useMemo<readonly FilterFieldKey[]>(
    () => Array.from(new Set<FilterFieldKey>(["type", ...lockedFields])),
    [lockedFields],
  );

  return (
    <WorkerProvider preload={["justified"]}>
      <AssetsProvider
        scopeId={scopeId}
        persist={false}
        defaultSelectionMode="single"
        initialState={{
          filters: {
            enabled: true,
            type: "PHOTO",
            raw: pickerInitialFilters.raw,
          },
        }}
      >
        <PhotoPickerContent
          onSelect={onSelect}
          title={title}
          initialFilters={pickerInitialFilters}
          lockedFields={pickerLockedFields}
        />
      </AssetsProvider>
    </WorkerProvider>
  );
}
