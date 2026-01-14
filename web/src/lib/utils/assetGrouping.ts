import { GroupByType } from "@/features/assets";
import { Asset } from "@/services";

/**
 * Groups assets by the specified criteria
 */
export const groupAssets = (
  assets: Asset[],
  groupBy: GroupByType,
): Record<string, Asset[]> => {
  if (!assets || assets.length === 0) {
    return {};
  }

  let grouped: Record<string, Asset[]> = {};

  switch (groupBy) {
    case "date":
      grouped = groupAssetsByDate(assets);
      break;
    case "type":
      grouped = groupAssetsByType(assets);
      break;
    case "album":
      grouped = groupAssetsByAlbum(assets);
      break;
    case "flat":
      // Flat mode: present all assets in a single unsectioned group
      grouped = { "All Results": assets };
      break;
    default:
      grouped = { "All Assets": assets };
  }

  return grouped;
};

/**
 * Groups assets by upload date
 */
const groupAssetsByDate = (assets: Asset[]): Record<string, Asset[]> => {
  const grouped: Record<string, Asset[]> = {};

  assets.forEach((asset) => {
    const date = asset.upload_time ? new Date(asset.upload_time) : new Date();
    const groupKey = formatDateGroupKey(date);

    if (!grouped[groupKey]) {
      grouped[groupKey] = [];
    }
    grouped[groupKey].push(asset);
  });

  return grouped;
};

/**
 * Groups assets by MIME type (e.g., image/jpeg, image/png).
 * Falls back to legacy asset.type buckets when MIME is missing.
 */
const groupAssetsByType = (assets: Asset[]): Record<string, Asset[]> => {
  const grouped: Record<string, Asset[]> = {};

  assets.forEach((asset) => {
    const mime = asset.mime_type?.trim();
    const groupKey =
      mime && mime.length > 0 ? mime : formatTypeGroupKey(asset.type || "");

    if (!grouped[groupKey]) {
      grouped[groupKey] = [];
    }
    grouped[groupKey].push(asset);
  });

  return grouped;
};

/**
 * Groups assets by album
 * TODO: Enable once albums property is added to AssetDTO in backend schema
 */
const groupAssetsByAlbum = (assets: Asset[]): Record<string, Asset[]> => {
  const grouped: Record<string, Asset[]> = {};

  // TODO: Uncomment once backend schema includes albums property
  // assets.forEach((asset) => {
  //   if (asset.albums && asset.albums.length > 0) {
  //     // Asset can be in multiple albums, add to each
  //     asset.albums.forEach((album: any) => {
  //       const groupKey = album.album_name || `Album ${album.album_id}`;
  //       if (!grouped[groupKey]) {
  //         grouped[groupKey] = [];
  //       }
  //       grouped[groupKey].push(asset);
  //     });
  //   } else {
  //     // Assets not in any album
  //     const groupKey = "No Album";
  //     if (!grouped[groupKey]) {
  //       grouped[groupKey] = [];
  //     }
  //     grouped[groupKey].push(asset);
  //   }
  // });

  // Temporary: Return empty grouping until albums property is available
  grouped["All Assets"] = assets;

  return grouped;
};

/**
 * Formats a date into a group key
 */
const formatDateGroupKey = (date: Date): string => {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);
  const thisWeekStart = new Date(
    today.getTime() - today.getDay() * 24 * 60 * 60 * 1000,
  );
  const thisMonthStart = new Date(now.getFullYear(), now.getMonth(), 1);
  const thisYearStart = new Date(now.getFullYear(), 0, 1);

  const assetDate = new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
  );

  if (assetDate.getTime() === today.getTime()) {
    return "Today";
  } else if (assetDate.getTime() === yesterday.getTime()) {
    return "Yesterday";
  } else if (assetDate >= thisWeekStart) {
    return "This Week";
  } else if (assetDate >= thisMonthStart) {
    return "This Month";
  } else if (assetDate >= thisYearStart) {
    return date.toLocaleDateString("en-US", { month: "long", year: "numeric" });
  } else {
    return date.getFullYear().toString();
  }
};

/**
 * Formats MIME or legacy type into a readable group key.
 * - If it looks like a MIME (contains "/"), return as-is
 * - Otherwise, map legacy types to wildcard MIME families
 */
const formatTypeGroupKey = (mimeOrLegacyType: string): string => {
  const val = (mimeOrLegacyType || "").trim();
  if (val.includes("/")) return val;

  switch (val.toUpperCase()) {
    case "PHOTO":
      return "image/*";
    case "VIDEO":
      return "video/*";
    case "AUDIO":
      return "audio/*";
    default:
      return "Unknown MIME";
  }
};

/**
 * Gets the flat array of assets from grouped assets in the correct order
 */
export const getFlatAssetsFromGrouped = (
  groupedAssets: Record<string, Asset[]>,
): Asset[] => {
  const flatAssets: Asset[] = [];
  Object.values(groupedAssets).forEach((assets) => {
    flatAssets.push(...assets);
  });
  return flatAssets;
};

/**
 * Finds the index of an asset in the flat array
 */
export const findAssetIndex = (assets: Asset[], assetId: string): number => {
  return assets.findIndex((asset) => asset.asset_id === assetId);
};

/**
 * Gets asset by ID from grouped assets
 */
export const getAssetById = (
  groupedAssets: Record<string, Asset[]>,
  assetId: string,
): Asset | null => {
  for (const assets of Object.values(groupedAssets)) {
    const asset = assets.find((a) => a.asset_id === assetId);
    if (asset) return asset;
  }
  return null;
};
