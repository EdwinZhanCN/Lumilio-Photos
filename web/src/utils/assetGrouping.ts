import { GroupByType, SortOrderType } from '@/hooks/page-hooks/usePhotosPageState';

/**
 * Groups assets by the specified criteria and applies sorting
 */
export const groupAssets = (
  assets: Asset[],
  groupBy: GroupByType,
  sortOrder: SortOrderType = 'desc'
): Record<string, Asset[]> => {
  if (!assets || assets.length === 0) {
    return {};
  }

  let grouped: Record<string, Asset[]> = {};

  switch (groupBy) {
    case 'date':
      grouped = groupAssetsByDate(assets);
      break;
    case 'type':
      grouped = groupAssetsByType(assets);
      break;
    case 'album':
      grouped = groupAssetsByAlbum(assets);
      break;
    default:
      grouped = { 'All Assets': assets };
  }

  // Sort assets within each group
  Object.keys(grouped).forEach(key => {
    grouped[key] = sortAssets(grouped[key], sortOrder);
  });

  // Sort group keys
  const sortedKeys = sortGroupKeys(Object.keys(grouped), groupBy, sortOrder);
  const sortedGrouped: Record<string, Asset[]> = {};
  sortedKeys.forEach(key => {
    sortedGrouped[key] = grouped[key];
  });

  return sortedGrouped;
};

/**
 * Groups assets by upload date
 */
const groupAssetsByDate = (assets: Asset[]): Record<string, Asset[]> => {
  const grouped: Record<string, Asset[]> = {};

  assets.forEach(asset => {
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
 * Groups assets by type (PHOTO, VIDEO, AUDIO, DOCUMENT)
 */
const groupAssetsByType = (assets: Asset[]): Record<string, Asset[]> => {
  const grouped: Record<string, Asset[]> = {};

  assets.forEach(asset => {
    const type = asset.type || 'Unknown';
    const groupKey = formatTypeGroupKey(type);

    if (!grouped[groupKey]) {
      grouped[groupKey] = [];
    }
    grouped[groupKey].push(asset);
  });

  return grouped;
};

/**
 * Groups assets by album
 */
const groupAssetsByAlbum = (assets: Asset[]): Record<string, Asset[]> => {
  const grouped: Record<string, Asset[]> = {};

  assets.forEach(asset => {
    if (asset.albums && asset.albums.length > 0) {
      // Asset can be in multiple albums, add to each
      asset.albums.forEach(album => {
        const groupKey = album.album_name || `Album ${album.album_id}`;
        if (!grouped[groupKey]) {
          grouped[groupKey] = [];
        }
        grouped[groupKey].push(asset);
      });
    } else {
      // Assets not in any album
      const groupKey = 'No Album';
      if (!grouped[groupKey]) {
        grouped[groupKey] = [];
      }
      grouped[groupKey].push(asset);
    }
  });

  return grouped;
};

/**
 * Sorts assets within a group
 */
const sortAssets = (assets: Asset[], sortOrder: SortOrderType): Asset[] => {
  return [...assets].sort((a, b) => {
    // Primary sort: upload time
    const dateA = a.upload_time ? new Date(a.upload_time).getTime() : 0;
    const dateB = b.upload_time ? new Date(b.upload_time).getTime() : 0;

    const dateDiff = sortOrder === 'desc' ? dateB - dateA : dateA - dateB;

    // Secondary sort: filename if dates are equal
    if (dateDiff === 0) {
      const nameA = a.original_filename || '';
      const nameB = b.original_filename || '';
      return sortOrder === 'desc'
        ? nameB.localeCompare(nameA)
        : nameA.localeCompare(nameB);
    }

    return dateDiff;
  });
};

/**
 * Sorts group keys based on grouping type
 */
const sortGroupKeys = (
  keys: string[],
  groupBy: GroupByType,
  sortOrder: SortOrderType
): string[] => {
  return [...keys].sort((a, b) => {
    if (groupBy === 'date') {
      // For date groups, sort by the date value
      const dateA = parseDateGroupKey(a);
      const dateB = parseDateGroupKey(b);
      const diff = dateB.getTime() - dateA.getTime();
      return sortOrder === 'desc' ? diff : -diff;
    } else {
      // For other groups, sort alphabetically
      return sortOrder === 'desc'
        ? b.localeCompare(a)
        : a.localeCompare(b);
    }
  });
};

/**
 * Formats a date into a group key
 */
const formatDateGroupKey = (date: Date): string => {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);
  const thisWeekStart = new Date(today.getTime() - (today.getDay() * 24 * 60 * 60 * 1000));
  const thisMonthStart = new Date(now.getFullYear(), now.getMonth(), 1);
  const thisYearStart = new Date(now.getFullYear(), 0, 1);

  const assetDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  if (assetDate.getTime() === today.getTime()) {
    return 'Today';
  } else if (assetDate.getTime() === yesterday.getTime()) {
    return 'Yesterday';
  } else if (assetDate >= thisWeekStart) {
    return 'This Week';
  } else if (assetDate >= thisMonthStart) {
    return 'This Month';
  } else if (assetDate >= thisYearStart) {
    return date.toLocaleDateString('en-US', { month: 'long', year: 'numeric' });
  } else {
    return date.getFullYear().toString();
  }
};

/**
 * Parses a date group key back to a Date object for sorting
 */
const parseDateGroupKey = (key: string): Date => {
  const now = new Date();

  switch (key) {
    case 'Today':
      return new Date(now.getFullYear(), now.getMonth(), now.getDate());
    case 'Yesterday':
      const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000);
      return new Date(yesterday.getFullYear(), yesterday.getMonth(), yesterday.getDate());
    case 'This Week':
      const thisWeek = new Date(now.getTime() - (now.getDay() * 24 * 60 * 60 * 1000));
      return new Date(thisWeek.getFullYear(), thisWeek.getMonth(), thisWeek.getDate());
    case 'This Month':
      return new Date(now.getFullYear(), now.getMonth(), 1);
    default:
      // Try to parse month/year or year
      const yearMatch = key.match(/^(\d{4})$/);
      if (yearMatch) {
        return new Date(parseInt(yearMatch[1]), 0, 1);
      }

      const monthYearMatch = key.match(/^(\w+)\s+(\d{4})$/);
      if (monthYearMatch) {
        const monthName = monthYearMatch[1];
        const year = parseInt(monthYearMatch[2]);
        const monthIndex = new Date(`${monthName} 1, ${year}`).getMonth();
        return new Date(year, monthIndex, 1);
      }

      return new Date(0); // Fallback
  }
};

/**
 * Formats asset type into a readable group key
 */
const formatTypeGroupKey = (type: string): string => {
  switch (type.toUpperCase()) {
    case 'PHOTO':
      return 'Photos';
    case 'VIDEO':
      return 'Videos';
    case 'AUDIO':
      return 'Audio';
    case 'DOCUMENT':
      return 'Documents';
    default:
      return 'Other';
  }
};

/**
 * Gets the flat array of assets from grouped assets in the correct order
 */
export const getFlatAssetsFromGrouped = (groupedAssets: Record<string, Asset[]>): Asset[] => {
  const flatAssets: Asset[] = [];
  Object.values(groupedAssets).forEach(assets => {
    flatAssets.push(...assets);
  });
  return flatAssets;
};

/**
 * Finds the index of an asset in the flat array
 */
export const findAssetIndex = (assets: Asset[], assetId: string): number => {
  return assets.findIndex(asset => asset.asset_id === assetId);
};

/**
 * Gets asset by ID from grouped assets
 */
export const getAssetById = (groupedAssets: Record<string, Asset[]>, assetId: string): Asset | null => {
  for (const assets of Object.values(groupedAssets)) {
    const asset = assets.find(a => a.asset_id === assetId);
    if (asset) return asset;
  }
  return null;
};
