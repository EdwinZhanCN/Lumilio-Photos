import { GroupByType } from "@/features/assets";
import PageHeader from "@/components/PageHeader";
import { FunnelIcon, PhotoIcon } from "@heroicons/react/24/outline";
import SearchBar from "@/components/SearchBar";
import FilterTool, { FilterDTO } from "./FilterTool";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";
import { useAssetsPageContext } from "@/features/assets";
import { useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage";

interface PhotosToolBarProps {
  groupBy: GroupByType;
  onGroupByChange: (groupBy: GroupByType) => void;
  onShowExifData?: (assetId: string) => void; // For EXIF data extraction
  onFiltersChange?: (filters: FilterDTO) => void;
}

const PhotosToolBar = ({
  groupBy,
  onGroupByChange,
  onFiltersChange,
}: PhotosToolBarProps) => {
  const { applyAdvancedFilter, resetFilters, setSearchQuery } =
    useAssetsContext();
  const { state: pageState } = useAssetsPageContext();
  const showMessage = useMessage();

  const handleSearchResults = useCallback(
    (results: Asset[]) => {
      showMessage("success", `Found ${results.length} results`);
    },
    [showMessage],
  );

  const handleSearchError = useCallback(
    (error: string) => {
      showMessage("error", `Search failed: ${error}`);
    },
    [showMessage],
  );

  const handleFiltersChange = useCallback(
    (filters: FilterDTO) => {
      onFiltersChange?.(filters);

      // Convert FilterDTO to AssetFilter format for the new API
      const assetFilter: any = {};

      // Only include non-empty filters
      if (filters.raw !== undefined) {
        assetFilter.raw = filters.raw;
      }
      if (filters.rating !== undefined) {
        assetFilter.rating = filters.rating;
      }
      if (filters.liked !== undefined) {
        assetFilter.liked = filters.liked;
      }
      if (filters.filename && filters.filename.value.trim()) {
        assetFilter.filename = {
          mode:
            filters.filename.operator === "starts_with"
              ? "startswith"
              : filters.filename.operator === "ends_with"
                ? "endswith"
                : filters.filename.operator,
          value: filters.filename.value.trim(),
        };
      }
      if (filters.date && (filters.date.from || filters.date.to)) {
        assetFilter.date = filters.date;
      }
      if (filters.camera_make && filters.camera_make.trim()) {
        assetFilter.camera_make = filters.camera_make.trim();
      }
      if (filters.lens && filters.lens.trim()) {
        assetFilter.lens = filters.lens.trim();
      }

      // 如果没有任何子过滤器启用（主开关或全部子项关闭），重置为未过滤状态
      if (Object.keys(assetFilter).length > 0) {
        // 有实际过滤条件 -> 进入过滤模式
        applyAdvancedFilter(assetFilter);
      } else {
        // 没有任何过滤条件 -> 仅退出过滤模式，但保持当前搜索（如果存在）
        resetFilters();
        if (pageState.searchQuery) {
          // 重新设置搜索查询以恢复搜索模式
          setSearchQuery(pageState.searchQuery);
        }
      }
    },
    [
      onFiltersChange,
      applyAdvancedFilter,
      resetFilters,
      pageState.searchQuery,
      setSearchQuery,
    ],
  );
  return (
    <PageHeader
      title="Photos"
      icon={<PhotoIcon className="w-6 h-6 text-primary" />}
    >
      {/* Group By Dropdown */}
      <div className="dropdown">
        <div tabIndex={0} role="button" className="btn btn-sm btn-ghost">
          <FunnelIcon className="size-4" />
          Group by {groupBy}
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
              Date
            </a>
          </li>
          <li>
            <a
              onClick={() => onGroupByChange("type")}
              className={groupBy === "type" ? "active" : ""}
            >
              Type
            </a>
          </li>
          <li>
            <a
              onClick={() => onGroupByChange("album")}
              className={groupBy === "album" ? "active" : ""}
            >
              Album
            </a>
          </li>
        </ul>
      </div>

      <FilterTool onChange={handleFiltersChange} autoApply={true} />
      <SearchBar
        onSearchResults={handleSearchResults}
        onSearchError={handleSearchError}
      />
    </PageHeader>
  );
};

export default PhotosToolBar;
