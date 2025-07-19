import { useState } from "react";
import SortDropDown from "./SortDropDown";
import {
  GroupByType,
  SortOrderType,
  ViewModeType,
} from "@/hooks/page-hooks/usePhotosPageState";
import {
  MagnifyingGlassIcon,
  Squares2X2Icon,
  ViewColumnsIcon,
  FunnelIcon,
  InformationCircleIcon,
} from "@heroicons/react/24/outline";

interface PhotosToolBarProps {
  groupBy: GroupByType;
  sortOrder: SortOrderType;
  viewMode: ViewModeType;
  searchQuery: string;
  onGroupByChange: (groupBy: GroupByType) => void;
  onSortOrderChange: (sortOrder: SortOrderType) => void;
  onViewModeChange: (viewMode: ViewModeType) => void;
  onSearchQueryChange: (query: string) => void;
  onShowExifData?: (assetId: string) => void; // For EXIF data extraction
}

const PhotosToolBar = ({
  groupBy,
  sortOrder,
  viewMode,
  searchQuery,
  onGroupByChange,
  onSortOrderChange,
  onViewModeChange,
  onSearchQueryChange,
  onShowExifData,
}: PhotosToolBarProps) => {
  const [showAdvancedFilters, setShowAdvancedFilters] = useState(false);

  return (
    <div className="space-y-4 mb-4">
      {/* Main Toolbar */}
      <div className="flex flex-wrap gap-2 items-center justify-between">
        <div className="flex items-center gap-2">
          <h1 className="text-2xl font-bold">Photos</h1>

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

          <SortDropDown
            sortOrder={sortOrder}
            onSortOrderChange={onSortOrderChange}
          />
        </div>

        <div className="flex items-center gap-2">
          {/* Search */}
          <div className="relative">
            <input
              type="text"
              placeholder="Search photos..."
              value={searchQuery}
              onChange={(e) => onSearchQueryChange(e.target.value)}
              className="input input-sm input-bordered w-48 pl-8"
            />
            <MagnifyingGlassIcon className="size-4 absolute left-2 top-1/2 transform -translate-y-1/2 text-gray-400" />
          </div>

          {/* View Mode Toggle */}
          <div className="join">
            <button
              className={`btn btn-sm join-item ${viewMode === "masonry" ? "btn-active" : "btn-ghost"}`}
              onClick={() => onViewModeChange("masonry")}
              title="Masonry View"
            >
              <ViewColumnsIcon className="size-4" />
            </button>
            <button
              className={`btn btn-sm join-item ${viewMode === "grid" ? "btn-active" : "btn-ghost"}`}
              onClick={() => onViewModeChange("grid")}
              title="Grid View"
            >
              <Squares2X2Icon className="size-4" />
            </button>
          </div>

          {/* Advanced Filters Toggle */}
          <button
            className={`btn btn-sm ${showAdvancedFilters ? "btn-active" : "btn-ghost"}`}
            onClick={() => setShowAdvancedFilters(!showAdvancedFilters)}
            title="Advanced Filters"
          >
            <FunnelIcon className="size-4" />
          </button>

          {/* EXIF Data Info */}
          {onShowExifData && (
            <div
              className="tooltip tooltip-bottom"
              data-tip="EXIF data available for selected photos"
            >
              <button className="btn btn-sm btn-ghost">
                <InformationCircleIcon className="size-4" />
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Advanced Filters Panel */}
      {showAdvancedFilters && (
        <div className="bg-base-200 rounded-lg p-4 space-y-3">
          <h3 className="text-sm font-semibold">Advanced Filters</h3>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
            {/* Date Range Filter - Placeholder */}
            <div>
              <label className="label label-text text-xs">Date Range</label>
              <div className="text-xs text-gray-500">
                🚧 Backend API pending - Will filter by upload/creation date
              </div>
            </div>

            {/* File Type Filter - Placeholder */}
            <div>
              <label className="label label-text text-xs">File Type</label>
              <div className="text-xs text-gray-500">
                🚧 Backend API pending - Will filter by PHOTO/VIDEO/etc.
              </div>
            </div>

            {/* File Size Filter - Placeholder */}
            <div>
              <label className="label label-text text-xs">File Size</label>
              <div className="text-xs text-gray-500">
                🚧 Backend API pending - Will filter by file size ranges
              </div>
            </div>

            {/* Camera/EXIF Filter - Placeholder */}
            <div>
              <label className="label label-text text-xs">Camera/EXIF</label>
              <div className="text-xs text-gray-500">
                🚧 Two implementations planned:
                <br />• Backend metadata (Asset.specificMetadata)
                <br />• Client-side extraction (useExtractExifdata hook)
              </div>
            </div>
          </div>

          {/* Usage Instructions */}
          <div className="bg-info/10 rounded p-3 text-xs">
            <strong>Usage Instructions:</strong>
            <ul className="list-disc list-inside mt-1 space-y-1">
              <li>
                <strong>Backend Metadata:</strong> Access via
                Asset.specificMetadata from API response
              </li>
              <li>
                <strong>Client Extraction:</strong> Use useExtractExifdata hook
                for full EXIF parsing
              </li>
              <li>
                <strong>Date Filters:</strong> Implement with backend API
                endpoints when ready
              </li>
              <li>
                <strong>Performance:</strong> Backend filtering preferred for
                large datasets
              </li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
};

export default PhotosToolBar;
