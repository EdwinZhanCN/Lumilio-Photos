import { SortByType } from "@/features/assets/types/assets.type";

export const isSortByType = (value: string | null): value is SortByType => {
  return value === "date_captured" || value === "recently_added";
};

export const getDefaultSortBy = (): SortByType => {
  return "date_captured";
};

export const resolveSortBy = (value?: string | null): SortByType => {
  const candidate = value ?? null;
  return isSortByType(candidate) ? candidate : getDefaultSortBy();
};
