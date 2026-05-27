import type { Asset } from "@/lib/assets/types";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type {
  AssetGroup,
  BrowseGroup,
  BrowseItem,
  BrowseStackItem,
  BrowseItemId,
  SortByType,
} from "@/features/assets/types/assets.type";
import { groupAssetsBySort } from "@/features/assets/utils/assetGroups";

export type BrowseItemDTO = components["schemas"]["dto.BrowseItemDTO"];

const isStackAsset = (asset: Asset): boolean =>
  Boolean(
    asset.asset_id &&
    asset.stack?.stack_id &&
    asset.stack?.stack_size &&
    asset.stack.stack_size > 1,
  );

const preferRepresentative = (current: Asset, candidate: Asset): Asset => {
  if (!current.stack?.stack_cover && candidate.stack?.stack_cover) {
    return candidate;
  }

  return current;
};

export const resolveStackFocusAssetId = (
  asset: Asset,
  stack?: BrowseStackItem,
): string | undefined => {
  const matchedMemberId = stack?.matchedMemberIds?.find((id) => Boolean(id));

  return matchedMemberId ?? asset.asset_id;
};

const toAssetItem = (asset: Asset): BrowseItem | null => {
  const assetId = asset.asset_id;
  if (!assetId) return null;

  return {
    type: "asset",
    id: `asset:${assetId}`,
    asset,
  };
};

export const getBrowseItemAsset = (item: BrowseItem): Asset =>
  item.type === "stack" ? item.representative : item.asset;

export const getBrowseItemAssetId = (item: BrowseItem): string | undefined =>
  getBrowseItemAsset(item).asset_id;

export const flattenBrowseGroupsToAssets = (groups?: BrowseGroup[]): Asset[] =>
  flattenBrowseGroups(groups).map(getBrowseItemAsset);

export const flattenBrowseGroups = (groups?: BrowseGroup[]): BrowseItem[] => {
  if (!groups || groups.length === 0) return [];
  return groups.flatMap((group) => group.items);
};

export const dedupeBrowseItemsById = (items: BrowseItem[]): BrowseItem[] => {
  const deduped: BrowseItem[] = [];
  const seen = new Set<string>();

  items.forEach((item) => {
    if (seen.has(item.id)) return;
    seen.add(item.id);
    deduped.push(item);
  });

  return deduped;
};

export const findBrowseItemById = (
  items: BrowseItem[],
  itemId: string,
): BrowseItem | undefined => items.find((item) => item.id === itemId);

export const findBrowseItemIndexByAssetId = (
  items: BrowseItem[],
  assetId: string,
): number =>
  items.findIndex((item) => {
    if (item.type === "asset") {
      return item.asset.asset_id === assetId;
    }

    if (getBrowseItemAsset(item).asset_id === assetId) {
      return true;
    }

    return (
      item.assets.some((a) => a.asset_id === assetId) ||
      item.memberAssetIds?.includes(assetId) === true
    );
  });

export const resolveSelectedBrowseItems = (
  selectedIds: Iterable<string>,
  items: BrowseItem[],
): BrowseItem[] => {
  const browseItemsById = new Map<BrowseItemId, BrowseItem>();

  items.forEach((item) => {
    browseItemsById.set(item.id, item);
  });

  return Array.from(selectedIds).flatMap((selectedId) => {
    const item = browseItemsById.get(selectedId as BrowseItemId);
    return item ? [item] : [];
  });
};

export type BrowseSelectionResolveMode = "visible" | "whole-stack";

export interface BrowseSelectionResolveOptions {
  stackMode?: BrowseSelectionResolveMode;
}

export const resolveBrowseSelectedAssetIds = (
  selectedIds: Iterable<string>,
  items: BrowseItem[],
  options: BrowseSelectionResolveOptions = {},
): string[] => {
  const stackMode = options.stackMode ?? "visible";
  const seen = new Set<string>();
  const resolved: string[] = [];

  const addAssetId = (assetId?: string) => {
    if (!assetId || seen.has(assetId)) return;
    seen.add(assetId);
    resolved.push(assetId);
  };

  resolveSelectedBrowseItems(selectedIds, items).forEach((item) => {
    if (item.type === "stack" && stackMode === "whole-stack") {
      const memberAssetIds = item.memberAssetIds?.filter(Boolean) ?? [];
      if (memberAssetIds.length > 0) {
        memberAssetIds.forEach(addAssetId);
        return;
      }
    }

    addAssetId(getBrowseItemAssetId(item));
  });

  return resolved;
};

export const createBrowseGroupsFromAssetGroups = (
  groups?: AssetGroup[],
): BrowseGroup[] => {
  if (!groups || groups.length === 0) return [];

  const stackItemsById = new Map<string, BrowseItem>();
  const stackGroupIndexById = new Map<string, number>();
  const browseGroups: BrowseGroup[] = [];

  groups.forEach((group) => {
    const items: BrowseItem[] = [];

    group.assets.forEach((asset) => {
      if (!asset.asset_id) return;

      if (!isStackAsset(asset)) {
        const assetItem = toAssetItem(asset);
        if (assetItem) items.push(assetItem);
        return;
      }

      const stackId = asset.stack?.stack_id;
      if (!stackId) return;

      const existingItem = stackItemsById.get(stackId);
      if (!existingItem || existingItem.type !== "stack") {
        const stackItem: BrowseItem = {
          type: "stack",
          id: `stack:${stackId}`,
          stackId,
          representative: asset,
          assets: [asset],
          memberAssetIds: asset.asset_id ? [asset.asset_id] : [],
          matchedMemberIds: asset.asset_id ? [asset.asset_id] : [],
        };
        stackItemsById.set(stackId, stackItem);
        stackGroupIndexById.set(stackId, browseGroups.length);
        items.push(stackItem);
        return;
      }

      existingItem.assets = [...existingItem.assets, asset];
      if (asset.asset_id) {
        existingItem.memberAssetIds = [
          ...(existingItem.memberAssetIds ?? []),
          asset.asset_id,
        ];
        existingItem.matchedMemberIds = [
          ...(existingItem.matchedMemberIds ?? []),
          asset.asset_id,
        ];
      }
      const nextRepresentative = preferRepresentative(
        existingItem.representative,
        asset,
      );
      if (nextRepresentative === existingItem.representative) {
        return;
      }

      existingItem.representative = nextRepresentative;
      const existingGroupIndex = stackGroupIndexById.get(stackId);
      if (existingGroupIndex === undefined) return;

      const existingGroup = browseGroups[existingGroupIndex];
      if (!existingGroup) return;

      existingGroup.items = existingGroup.items.filter(
        (item) => item.id !== existingItem.id,
      );
      items.push(existingItem);
      stackGroupIndexById.set(stackId, browseGroups.length);
    });

    if (items.length === 0) return;
    browseGroups.push({ key: group.key, items });
  });

  return browseGroups.filter((group) => group.items.length > 0);
};

export const createBrowseGroupsFromAssets = (
  assets?: Asset[],
  key = "flat:all",
): BrowseGroup[] =>
  createBrowseGroupsFromAssetGroups(
    assets && assets.length > 0 ? [{ key, assets }] : [],
  );

export const createBrowseItemsFromBrowseItemDTOs = (
  dtoItems?: BrowseItemDTO[] | null,
): BrowseItem[] => {
  if (!dtoItems || dtoItems.length === 0) return [];

  const items: BrowseItem[] = [];

  dtoItems.forEach((item) => {
    if (item.type === "stack" && item.stack?.cover_asset) {
      const representative = item.stack.cover_asset as Asset;
      const stackId = item.stack.stack_id;
      if (!representative.asset_id || !stackId) return;

      items.push({
        type: "stack",
        id: `stack:${stackId}`,
        stackId,
        representative,
        assets: [representative],
        memberAssetIds: item.stack.member_asset_ids ?? [],
        matchedMemberIds: item.stack.matched_member_ids ?? [],
      });
      return;
    }

    if (item.type === "asset" && item.asset?.asset_id) {
      items.push({
        type: "asset",
        id: `asset:${item.asset.asset_id}`,
        asset: item.asset as Asset,
      });
    }
  });

  return items;
};

export const createBrowseGroupsFromBrowseItemDTOs = (
  dtoItems?: BrowseItemDTO[] | null,
  key = "flat:all",
): BrowseGroup[] => {
  const items = createBrowseItemsFromBrowseItemDTOs(dtoItems);
  return items.length > 0 ? [{ key, items }] : [];
};

export const groupBrowseItemsBySort = (
  items: BrowseItem[],
  sortBy: SortByType,
): BrowseGroup[] => {
  if (items.length === 0) return [];

  const itemByRepresentativeId = new Map<string, BrowseItem>();
  items.forEach((item) => {
    const assetId = getBrowseItemAsset(item).asset_id;
    if (!assetId) return;
    itemByRepresentativeId.set(assetId, item);
  });

  return groupAssetsBySort(items.map(getBrowseItemAsset), sortBy)
    .map((group) => ({
      key: group.key,
      items: group.assets.flatMap((asset) => {
        const assetId = asset.asset_id;
        if (!assetId) return [];
        const mapped = itemByRepresentativeId.get(assetId);
        return mapped ? [mapped] : [];
      }),
    }))
    .filter((group) => group.items.length > 0);
};

export const mergeAdjacentBrowseGroups = (
  ...groupCollections: BrowseGroup[][]
): BrowseGroup[] => {
  const merged: BrowseGroup[] = [];

  groupCollections.forEach((groups) => {
    groups.forEach((group) => {
      const previous = merged[merged.length - 1];
      if (previous && previous.key === group.key) {
        previous.items = [...previous.items, ...group.items];
        return;
      }
      merged.push({ key: group.key, items: [...group.items] });
    });
  });

  return merged;
};

export const browseGroupsFromQueryLikePage = (params: {
  items?: BrowseItemDTO[] | null;
  sortBy: SortByType;
}): BrowseGroup[] => {
  const fromDto = createBrowseItemsFromBrowseItemDTOs(params.items);
  return groupBrowseItemsBySort(fromDto, params.sortBy);
};

export const browseGroupsFromSearchTop = (params: {
  topItems?: BrowseItemDTO[] | null;
}): BrowseGroup[] => {
  const fromDto = createBrowseItemsFromBrowseItemDTOs(params.topItems);
  return fromDto.length > 0 ? [{ key: "search:top_results", items: fromDto }] : [];
};

export const browseGroupsFromSearchResultsPage = (params: {
  resultItems?: BrowseItemDTO[] | null;
}): BrowseGroup[] => {
  const fromDto = createBrowseItemsFromBrowseItemDTOs(params.resultItems);
  return fromDto.length > 0 ? [{ key: "search:results", items: fromDto }] : [];
};

export const countLoadedBrowseRowsFromPage = (params: {
  items?: BrowseItemDTO[] | null;
}): number => {
  const fromDto = createBrowseItemsFromBrowseItemDTOs(params.items);
  return fromDto.length;
};
