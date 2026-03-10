import type { TFunction } from "i18next";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { Asset } from "@/lib/assets/types";
import type { AssetGroup } from "@/features/assets/types/assets.type";

type AssetGroupDTO = components["schemas"]["dto.AssetGroupDTO"];

export const DEFAULT_GROUP_KEYS = new Set(["flat:all"]);

const normalizeVisibleAssets = (assets: Asset[]): Asset[] =>
  assets.filter((asset) => !asset.is_deleted && !asset.deleted_at);

export const normalizeAssetGroups = (
  groups?: AssetGroupDTO[],
): AssetGroup[] => {
  if (!Array.isArray(groups)) return [];

  return groups
    .map((group) => ({
      key: group.key ?? "flat:all",
      assets: normalizeVisibleAssets(group.assets ?? []),
    }))
    .filter((group) => group.assets.length > 0);
};

export const mergeAdjacentAssetGroups = (
  ...collections: AssetGroup[][]
): AssetGroup[] => {
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

export const formatAssetGroupLabel = (
  key: string,
  t: TFunction,
  locale: string,
) => {
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
