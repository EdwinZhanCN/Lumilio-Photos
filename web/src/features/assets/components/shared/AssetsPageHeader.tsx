import PageHeader from "@/components/PageHeader";
import {
  PhotoIcon,
  VideoCameraIcon,
  MusicalNoteIcon,
} from "@heroicons/react/24/outline";
import { SquareMousePointer, FunnelIcon } from "lucide-react";
import SearchBar from "@/components/SearchBar";
import FilterTool, {
  FilterDTO,
} from "@/features/assets/components/Photos/PhotosToolBar/FilterTool";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";
import { useSelection } from "@/features/assets/hooks/useSelection";
import { GroupByType } from "@/features/assets/types";
import { selectTabTitle } from "@/features/assets/reducers/ui.reducer";
import { useCallback, useMemo, useRef, useEffect, useState } from "react";

interface AssetsPageHeaderProps {
  groupBy: GroupByType;
  onGroupByChange: (groupBy: GroupByType) => void;
  onFiltersChange?: (filters: FilterDTO) => void;
}

const AssetsPageHeader = ({
  groupBy,
  onGroupByChange,
  onFiltersChange,
}: AssetsPageHeaderProps) => {
  const { state, dispatch } = useAssetsContext();
  const selection = useSelection();

  const currentTab = state.ui.currentTab;
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

  // Handle search results

  // Handle search query changes
  const [debouncedValue, setDebouncedValue] = useState(state.ui.searchQuery);
  const handleSearchQueryChange = useCallback((query: string) => {
    setDebouncedValue(query);
  }, []);

  // Debounce effect to dispatch search query changes
  useEffect(() => {
    const timer = setTimeout(() => {
      if (debouncedValue !== state.ui.searchQuery) {
        dispatch({ type: "SET_SEARCH_QUERY", payload: debouncedValue });
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [debouncedValue, state.ui.searchQuery, dispatch]);

  // Handle search activation (switch to flat view when searching)
  const handleSearchActivationChange = useCallback(
    (active: boolean) => {
      if (active && groupBy !== "flat") {
        onGroupByChange("flat");
      }
    },
    [groupBy, onGroupByChange],
  );

  // Use ref to store the latest onFiltersChange callback to avoid dependency issues
  const onFiltersChangeRef = useRef(onFiltersChange);
  const onGroupByChangeRef = useRef(onGroupByChange);

  useEffect(() => {
    onFiltersChangeRef.current = onFiltersChange;
    onGroupByChangeRef.current = onGroupByChange;
  });

  // Handle filter changes
  const handleFiltersChange = useCallback(
    (filters: FilterDTO) => {
      onFiltersChangeRef.current?.(filters);

      if (filters.raw !== undefined) {
        dispatch({ type: "SET_FILTER_RAW", payload: filters.raw });
      }
      if (filters.rating !== undefined) {
        dispatch({ type: "SET_FILTER_RATING", payload: filters.rating });
      }
      if (filters.liked !== undefined) {
        dispatch({ type: "SET_FILTER_LIKED", payload: filters.liked });
      }
      if (filters.filename && filters.filename.value.trim()) {
        dispatch({
          type: "SET_FILTER_FILENAME",
          payload: {
            mode:
              filters.filename.operator === "starts_with"
                ? "startswith"
                : filters.filename.operator === "ends_with"
                  ? "endswith"
                  : filters.filename.operator,
            value: filters.filename.value.trim(),
          },
        });
      }
      if (filters.date && (filters.date.from || filters.date.to)) {
        dispatch({ type: "SET_FILTER_DATE", payload: filters.date });
      }
      if (filters.camera_make && filters.camera_make.trim()) {
        dispatch({
          type: "SET_FILTER_CAMERA_MAKE",
          payload: filters.camera_make.trim(),
        });
      }
      if (filters.lens && filters.lens.trim()) {
        dispatch({ type: "SET_FILTER_LENS", payload: filters.lens.trim() });
      }

      // Enable filters if any are set
      const hasFilters = Object.keys(filters).length > 0;
      dispatch({ type: "SET_FILTERS_ENABLED", payload: hasFilters });

      // Switch to flat view when filtering for better search/filter result visibility
      if (hasFilters && groupBy !== "flat") {
        onGroupByChangeRef.current("flat");
      }
    },
    [dispatch, groupBy],
  );

  // Toggle selection mode
  const handleToggleSelection = useCallback(() => {
    selection.setEnabled(!selection.enabled);
  }, [selection]);

  return (
    <PageHeader
      title={tabTitle}
      icon={<TabIcon className="w-6 h-6 text-primary" />}
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
          <li>
            <a
              onClick={() => onGroupByChange("flat")}
              className={groupBy === "flat" ? "active" : ""}
            >
              Flat
            </a>
          </li>
        </ul>
      </div>

      {/* Filter Tool */}
      <FilterTool onChange={handleFiltersChange} autoApply={true} />

      {/* Selection Toggle Button */}
      <button
        className={`btn btn-sm btn-circle btn-soft ${
          selection.enabled ? "btn-primary" : "btn-info"
        } relative`}
        onClick={handleToggleSelection}
        title={
          selection.enabled ? "Exit Selection Mode" : "Enter Selection Mode"
        }
      >
        <SquareMousePointer className="w-4 h-4" />
        {selection.selectedCount > 0 && (
          <span className="badge badge-xs badge-primary absolute -right-1 -top-1">
            {selection.selectedCount}
          </span>
        )}
      </button>

      {/* Search Bar */}
      <SearchBar
        value={debouncedValue}
        onChange={handleSearchQueryChange}
        onActivationChange={handleSearchActivationChange}
        placeholder={`Search ${tabTitle.toLowerCase()}...`}
        enableSemanticSearch={currentTab === "photos"}
      />
    </PageHeader>
  );
};

export default AssetsPageHeader;
