import { GroupByType } from "@/features/assets/types/assets.type";

export const isGroupByType = (value: string | null): value is GroupByType => {
  return value === "date" || value === "type" || value === "flat";
};

export const getDefaultGroupBy = (_layout?: string): GroupByType => {
  return "date";
};

export const resolveGroupByFromUrl = (
  value: string | null,
  layout?: string,
): GroupByType => {
  return isGroupByType(value) ? value : getDefaultGroupBy(layout);
};
