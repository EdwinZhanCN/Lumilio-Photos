import { useEffect, useMemo } from "react";
import { Image as ImageIcon } from "lucide-react";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import {
  useSortBy,
  useUIActions,
  useFilterActions,
  useSelectionActions,
} from "@/features/assets/selectors";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import { useSelection } from "@/features/assets/hooks/useSelection";
import SquareGallery from "@/features/assets/components/page/SquareGallery/SquareGallery";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { flattenAssetGroups } from "@/features/assets/utils/assetGroups";
import { useI18n } from "@/lib/i18n";

type PhotoPickerContentProps = {
  onSelect: (id: string) => void;
  title?: string;
};

type PhotoPickerProps = PhotoPickerContentProps & {
  scopeId: string;
};

function PhotoPickerContent({
  onSelect,
  title,
}: PhotoPickerContentProps): React.JSX.Element {
  const { t } = useI18n();
  const sortBy = useSortBy();
  const { setSortBy, setSearchQuery } = useUIActions();
  const { resetFilters, batchUpdateFilters } = useFilterActions();
  const { clear: clearSelection, setEnabled: setSelectionEnabled } =
    useSelectionActions();
  const selection = useSelection();

  const {
    assets: allAssets,
    groups: groupedAssets,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    viewKey,
  } = useCurrentTabAssets({
    withGroups: true,
    sortBy,
  });

  const flatAssets = useMemo(() => {
    if (groupedAssets && groupedAssets.length > 0) {
      return flattenAssetGroups(groupedAssets);
    }
    return allAssets;
  }, [groupedAssets, allAssets]);

  const layoutKey = useMemo(() => {
    const assetIds = flatAssets
      .map((asset) => asset.asset_id)
      .filter((id): id is string => Boolean(id));
    return `${viewKey}:${assetIds.join(",")}`;
  }, [viewKey, flatAssets]);

  useEffect(() => {
    clearSelection();
    resetFilters();
    batchUpdateFilters({
      enabled: true,
      type: "PHOTO",
    });
    setSearchQuery("");
    setSelectionEnabled(true);
  }, [
    batchUpdateFilters,
    clearSelection,
    resetFilters,
    setSearchQuery,
    setSelectionEnabled,
  ]);

  useEffect(() => {
    if (selection.enabled && selection.selectedCount > 0) {
      const id = Array.from(selection.selectedIds)[0];
      if (id) {
        onSelect(id as string);
      }
    }
  }, [selection.selectedIds, selection.selectedCount, selection.enabled, onSelect]);

  return (
    <div className="flex h-full flex-col overflow-hidden bg-base-100">
      <AssetsPageHeader
        sortBy={sortBy}
        onSortByChange={setSortBy}
        title={
          title ??
          t("collections.createModal.coverPicker.title", {
            defaultValue: "Pick a photo",
          })
        }
        icon={<ImageIcon className="h-6 w-6 text-primary" />}
      />
      <div className="custom-scrollbar flex-1 overflow-x-hidden overflow-y-auto">
        <SquareGallery
          groups={groupedAssets || []}
          key={layoutKey}
          openCarousel={() => {}}
          onLoadMore={fetchMore}
          hasMore={hasMore}
          isLoadingMore={isLoadingMore}
          isLoading={isLoading && allAssets.length === 0}
          columns={5}
        />
      </div>
    </div>
  );
}

export default function PhotoPicker({
  scopeId,
  onSelect,
  title,
}: PhotoPickerProps): React.JSX.Element {
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
          },
        }}
      >
        <PhotoPickerContent onSelect={onSelect} title={title} />
      </AssetsProvider>
    </WorkerProvider>
  );
}
