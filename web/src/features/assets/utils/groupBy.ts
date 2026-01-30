import { GroupByType } from "@/features/assets/types/assets.type";

export const isGroupByType = (value: string | null): value is GroupByType => {
  return (
    value === "date" ||
    value === "type" ||
    value === "album" ||
    value === "flat"
  );
};

export const getDefaultGroupBy = (layout?: string): GroupByType => {
  return layout === "wide" ? "type" : "date";
};

export const resolveGroupByFromUrl = (
  value: string | null,
  layout?: string,
): GroupByType => {
  return isGroupByType(value) ? value : getDefaultGroupBy(layout);
};
