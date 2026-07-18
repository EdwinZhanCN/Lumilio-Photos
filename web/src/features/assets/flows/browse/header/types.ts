import type { ReactNode } from "react";
import type { AssetsBulkActionId, AssetsBulkActionInput } from "@/lib/assets/bulkActions";
import type { BrowseItem, SortByType } from "../../../types";
import type { AssetBrowseConstraint, AssetUserFilter } from "../../../model/filter";

export type ConfirmableBulkAction =
  | { type: "rating"; rating: number }
  | { type: "liked"; liked: boolean };

export interface AssetsPageHeaderProps {
  sortBy: SortByType;
  onSortByChange: (sortBy: SortByType) => void;
  filter: AssetUserFilter;
  constraint?: AssetBrowseConstraint;
  onFiltersChange: (filters: AssetUserFilter) => void;
  title?: string;
  subtitle?: string;
  icon?: ReactNode;
  browseItems?: BrowseItem[];
  bulkActions?: AssetsBulkActionInput;
  hiddenBulkActions?: readonly AssetsBulkActionId[];
  capabilities?: {
    showScan?: boolean;
  };
  scopeControlHidden?: boolean;
}
