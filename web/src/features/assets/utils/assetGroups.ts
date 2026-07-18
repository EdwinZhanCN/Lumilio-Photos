import type { TFunction } from "i18next";
import type { Asset } from "@/lib/assets/types";
import type { AssetGroup, SortByType } from "../types";

export const DEFAULT_GROUP_KEYS = new Set<string>();

const parseAssetDate = (value?: string | null): Date => {
  if (!value) return new Date(0);

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return new Date(0);
  }

  return parsed;
};

const getSortDate = (asset: Asset, sortBy: SortByType): Date => {
  if (sortBy === "recently_added") {
    return parseAssetDate(asset.upload_time);
  }

  return parseAssetDate(asset.taken_time ?? asset.upload_time);
};

type CalendarParts = {
  year: number;
  month: number;
  day: number;
  weekday: number;
};

const getCalendarParts = (date: Date, offsetMinutes?: number): CalendarParts => {
  if (typeof offsetMinutes === "number") {
    const shifted = new Date(date.getTime() + offsetMinutes * 60_000);
    return {
      year: shifted.getUTCFullYear(),
      month: shifted.getUTCMonth() + 1,
      day: shifted.getUTCDate(),
      weekday: shifted.getUTCDay(),
    };
  }

  return {
    year: date.getFullYear(),
    month: date.getMonth() + 1,
    day: date.getDate(),
    weekday: date.getDay(),
  };
};

const createBoundaryDate = (parts: CalendarParts, useUTC: boolean): Date => {
  if (useUTC) {
    return new Date(Date.UTC(parts.year, parts.month - 1, parts.day));
  }

  return new Date(parts.year, parts.month - 1, parts.day);
};

const addDays = (date: Date, days: number, useUTC: boolean): Date => {
  const next = new Date(date.getTime());
  if (useUTC) {
    next.setUTCDate(next.getUTCDate() + days);
  } else {
    next.setDate(next.getDate() + days);
  }
  return next;
};

const buildDateGroupKey = (assetDate: Date, now: Date, offsetMinutes?: number): string => {
  const useUTC = typeof offsetMinutes === "number";
  const assetParts = getCalendarParts(assetDate, offsetMinutes);
  const nowParts = getCalendarParts(now, offsetMinutes);

  const assetDay = createBoundaryDate(assetParts, useUTC);
  const today = createBoundaryDate(nowParts, useUTC);
  const yesterday = addDays(today, -1, useUTC);
  const thisWeekStart = addDays(today, -nowParts.weekday, useUTC);
  const thisMonthStart = useUTC
    ? new Date(Date.UTC(nowParts.year, nowParts.month - 1, 1))
    : new Date(nowParts.year, nowParts.month - 1, 1);
  const assetStamp = assetDay.getTime();

  switch (true) {
    case assetStamp === today.getTime():
      return "date:today";
    case assetStamp === yesterday.getTime():
      return "date:yesterday";
    case assetStamp >= thisWeekStart.getTime():
      return "date:this_week";
    case assetStamp >= thisMonthStart.getTime():
      return "date:this_month";
    default:
      return `date:month:${assetParts.year.toString().padStart(4, "0")}-${assetParts.month
        .toString()
        .padStart(2, "0")}`;
  }
};

const getAssetGroupKey = (asset: Asset, sortBy: SortByType, now: Date): string => {
  const sortDate = getSortDate(asset, sortBy);
  const captureOffsetMinutes =
    sortBy === "date_captured" ? asset.capture_offset_minutes : undefined;

  return buildDateGroupKey(sortDate, now, captureOffsetMinutes);
};

export const groupAssetsBySort = (
  assets: Asset[],
  sortBy: SortByType,
  now: Date = new Date(),
): AssetGroup[] => {
  if (assets.length === 0) {
    return [];
  }

  const groups: AssetGroup[] = [];
  assets.forEach((asset) => {
    const key = getAssetGroupKey(asset, sortBy, now);
    const last = groups[groups.length - 1];
    if (last && last.key === key) {
      last.assets = [...last.assets, asset];
      return;
    }

    groups.push({
      key,
      assets: [asset],
    });
  });

  return groups;
};

export const mergeAdjacentAssetGroups = (...collections: AssetGroup[][]): AssetGroup[] => {
  const merged: AssetGroup[] = [];

  collections.forEach((collection) => {
    collection.forEach((group) => {
      if (group.assets.length === 0) return;

      const last = merged[merged.length - 1];
      if (last && last.key === group.key) {
        last.assets = [...last.assets, ...group.assets];
        return;
      }

      merged.push({
        key: group.key,
        assets: [...group.assets],
      });
    });
  });

  return merged;
};

export const flattenAssetGroups = (groups?: AssetGroup[]): Asset[] => {
  if (!groups || groups.length === 0) return [];
  return groups.flatMap((group) => group.assets);
};

export const findAssetIndex = (assets: Asset[], assetId: string): number =>
  assets.findIndex((asset) => asset.asset_id === assetId);

export const getViewerTimeZone = () => {
  if (typeof Intl === "undefined") return "UTC";
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
};

export const formatAssetGroupLabel = (key: string, t: TFunction, locale: string) => {
  switch (key) {
    case "date:today":
      return t("assets.groups.today", { defaultValue: "Today" });
    case "date:yesterday":
      return t("assets.groups.yesterday", { defaultValue: "Yesterday" });
    case "date:this_week":
      return t("assets.groups.thisWeek", { defaultValue: "This Week" });
    case "date:this_month":
      return t("assets.groups.thisMonth", { defaultValue: "This Month" });
    case "search:top_results":
      return t("search.topResults", { defaultValue: "Top Results" });
    case "search:results":
      return t("search.results", { defaultValue: "Results" });
    case "flat:all":
      return t("assets.groups.allResults", { defaultValue: "All Results" });
    default:
      break;
  }

  if (key.startsWith("date:month:")) {
    const raw = key.slice("date:month:".length);
    const [yearStr, monthStr] = raw.split("-");
    const year = Number(yearStr);
    const month = Number(monthStr);
    if (!Number.isNaN(year) && !Number.isNaN(month)) {
      return new Intl.DateTimeFormat(locale || undefined, {
        month: "long",
        year: "numeric",
        timeZone: "UTC",
      }).format(new Date(Date.UTC(year, month - 1, 1)));
    }
  }

  if (key.startsWith("date:year:")) {
    return key.slice("date:year:".length);
  }

  if (key.startsWith("type:")) {
    return key.slice("type:".length);
  }

  return key;
};
