import type { ReactNode } from "react";
import type { AssetsBulkActionId, AssetsBulkActionInput } from "@/lib/assets/bulkActions";
import type { FilterDTO, FilterFieldKey } from "../../page/FilterTool/types";
import type { BrowseItem, SortByType } from "../../../types/assets.type";

export type ConfirmableBulkAction =
  | { type: "rating"; rating: number }
  | { type: "liked"; liked: boolean };

export interface AssetsPageHeaderProps {
  sortBy: SortByType;
  onSortByChange: (sortBy: SortByType) => void;
  onFiltersChange?: (filters: FilterDTO) => void;
  title?: string;
  subtitle?: string;
  icon?: ReactNode;
  browseItems?: BrowseItem[];
  lockedFilterFields?: readonly FilterFieldKey[];
  bulkActions?: AssetsBulkActionInput;
  hiddenBulkActions?: readonly AssetsBulkActionId[];
  capabilities?: {
    showScan?: boolean;
  };
  scopeControlHidden?: boolean;
}
