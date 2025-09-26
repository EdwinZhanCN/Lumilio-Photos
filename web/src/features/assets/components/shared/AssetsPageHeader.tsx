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
import { useCallback, useMemo, useRef, useEffect } from "react";

interface AssetsPageHeaderProps {
  groupBy: GroupByType;
  onGroupByChange: (groupBy: GroupByType) => void;
  onFiltersChange?: (filters: FilterDTO) => void;
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
}: AssetsPageHeaderProps) => {
  const { state, dispatch } = useAssetsContext();
  const selection = useSelection();

  // Hydrate FilterTool from global filters (single source of truth)
  const inboundDTO = useMemo(() => {
    const f = state.filters;
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
  }, [state.filters]);
  const inboundHash = useMemo(
    () => JSON.stringify(inboundDTO || {}),
    [inboundDTO],
  );

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

  // Handle search activation (switch to flat view when searching)
  useEffect(() => {
    if (state.ui.searchQuery.trim() && groupBy !== "flat") {
      onGroupByChange("flat");
    }
  }, [state.ui.searchQuery, groupBy, onGroupByChange]);

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
      // Prevent re-emit loop when FilterTool mounts with initial values
      const nextHash = JSON.stringify(filters || {});
      if (nextHash === inboundHash) {
        onFiltersChangeRef.current?.(filters);
        return;
      }

      // Build a full payload resetting all fields first (single source of truth)
      const payload: any = {
        enabled: Object.keys(filters).length > 0,
        raw: undefined,
        rating: undefined,
        liked: undefined,
        filename: undefined,
        date: undefined,
        camera_make: undefined,
        lens: undefined,
      };

      if (filters.raw !== undefined) payload.raw = filters.raw;
      if (filters.rating !== undefined) payload.rating = filters.rating;
      if (filters.liked !== undefined) payload.liked = filters.liked;

      if (filters.filename && filters.filename.value.trim()) {
        payload.filename = {
          mode: mapFilenameOperatorToMode(filters.filename.operator),
          value: filters.filename.value.trim(),
        };
      }

      if (filters.date && (filters.date.from || filters.date.to)) {
        payload.date = {
          from: filters.date.from,
          to: filters.date.to,
        };
      }

      if (filters.camera_make && filters.camera_make.trim()) {
        payload.camera_make = filters.camera_make.trim();
      }

      if (filters.lens && filters.lens.trim()) {
        payload.lens = filters.lens.trim();
      }

      dispatch({ type: "BATCH_UPDATE_FILTERS", payload });

      // Switch to flat view when filtering for better result visibility
      if (payload.enabled && groupBy !== "flat") {
        onGroupByChangeRef.current("flat");
      }

      onFiltersChangeRef.current?.(filters);
    },
    [dispatch, groupBy, inboundHash],
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
        <div
          tabIndex={0}
          role="button"
          className="btn btn-sm btn-soft btn-info"
        >
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
      <FilterTool
        initial={inboundDTO}
        onChange={handleFiltersChange}
        autoApply={true}
      />

      {/* Selection Toggle Button */}
      <button
        className={`btn btn-sm btn-circle btn-soft btn-info ${
          selection.enabled ? "btn-active" : ""
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
      <SearchBar enableSemanticSearch={currentTab === "photos"} />
    </PageHeader>
  );
};

export default AssetsPageHeader;
