import { SortOrderType } from "@/hooks/page-hooks/useAssetsPageState";
import {
  BarsArrowUpIcon,
  BarsArrowDownIcon,
} from "@heroicons/react/24/outline";

interface SortDropDownProps {
  sortOrder: SortOrderType;
  onSortOrderChange: (sortOrder: SortOrderType) => void;
}

export default function SortDropDown({
  sortOrder,
  onSortOrderChange,
}: SortDropDownProps) {
  return (
    <div className="dropdown">
      <div tabIndex={0} role="button" className="btn btn-sm btn-ghost">
        {sortOrder === "desc" ? (
          <BarsArrowDownIcon className="size-4" />
        ) : (
          <BarsArrowUpIcon className="size-4" />
        )}
        Sort {sortOrder === "desc" ? "Newest" : "Oldest"}
      </div>
      <ul
        tabIndex={0}
        className="dropdown-content menu bg-base-200 rounded-box z-[1] w-40 p-2 shadow"
      >
        <li>
          <a
            onClick={() => onSortOrderChange("desc")}
            className={sortOrder === "desc" ? "active" : ""}
          >
            <BarsArrowDownIcon className="size-4" />
            Newest First
          </a>
        </li>
        <li>
          <a
            onClick={() => onSortOrderChange("asc")}
            className={sortOrder === "asc" ? "active" : ""}
          >
            <BarsArrowUpIcon className="size-4" />
            Oldest First
          </a>
        </li>
      </ul>
    </div>
  );
}
