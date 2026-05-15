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
import { useCurrentAssetsView } from "@/features/assets/hooks/useAssetsView";
import { useSelection } from "@/features/assets/hooks/useSelection";
import SquareGallery from "@/features/assets/components/page/SquareGallery/SquareGallery";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { resolveBrowseSelectedAssetIds } from "@/features/assets/utils/browseItems";
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
    browseGroups,
    browseItems,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    viewKey,
  } = useCurrentAssetsView({
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
      const id = resolveBrowseSelectedAssetIds(selection.selectedIds, browseItems)[0];
      if (id) {
        onSelect(id);
      }
    }
  }, [
    browseItems,
    selection.selectedIds,
    selection.selectedCount,
    selection.enabled,
    onSelect,
  ]);

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
