import type { Asset } from "@/lib/assets/types";
import type {
  AssetGroup,
  BrowseGroup,
  BrowseItem,
} from "@/features/assets/types/assets.type";

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

export const findBrowseItemIndexByAssetId = (
  items: BrowseItem[],
  assetId: string,
): number =>
  items.findIndex((item) => {
    if (item.type === "asset") {
      return item.asset.asset_id === assetId;
    }

    return item.assets.some((asset) => asset.asset_id === assetId);
  });

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
        };
        stackItemsById.set(stackId, stackItem);
        stackGroupIndexById.set(stackId, browseGroups.length);
        items.push(stackItem);
        return;
      }

      existingItem.assets = [...existingItem.assets, asset];
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
