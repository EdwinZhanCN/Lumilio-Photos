import { GroupByType } from "@/features/assets";
import PageHeader from "@/components/PageHeader";
import { FunnelIcon, PhotoIcon } from "@heroicons/react/24/outline";
import SearchBar from "@/components/SearchBar";
import FilterTool from "./FilterTool";

interface PhotosToolBarProps {
  groupBy: GroupByType;
  searchQuery: string;
  onGroupByChange: (groupBy: GroupByType) => void;
  onSearchQueryChange: (query: string) => void;
  onShowExifData?: (assetId: string) => void; // For EXIF data extraction
}

const PhotosToolBar = ({
  groupBy,
  searchQuery,
  onGroupByChange,
  onSearchQueryChange,
}: PhotosToolBarProps) => {
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

      {/* Search */}
      <div className="relative">
        <input
          type="text"
          placeholder="Filter photos..."
          value={searchQuery}
          onChange={(e) => onSearchQueryChange(e.target.value)}
          className="input input-sm input-bordered"
        />
      </div>
      <FilterTool />
      <SearchBar />
    </PageHeader>
  );
};

export default PhotosToolBar;
