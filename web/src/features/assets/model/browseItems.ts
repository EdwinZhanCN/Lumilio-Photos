import type { Asset, StackPreview } from "@/lib/assets/types";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type {
  AssetGroup,
  BrowseGroup,
  BrowseItem,
  BrowseMediaItem,
  BrowseStackItem,
  BrowseStackKind,
  BrowseStackMemberRef,
  BrowseItemId,
  MediaCompositionFacts,
  SortByType,
} from "../types";
import { getAssetGroupKey } from "./assetGroups";

export type BrowseItemDTO = components["schemas"]["dto.BrowseItemDTO"];
type BrowseStackMemberDTO = components["schemas"]["dto.BrowseStackMemberDTO"];
type MediaCompositionDTO = components["schemas"]["dto.MediaCompositionDTO"];

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
  const matchedMemberId = stack?.matchedMembers.find((member) =>
    Boolean(member.primaryAssetId),
  )?.primaryAssetId;

  return matchedMemberId ?? asset.asset_id;
};

const toMediaItemFromAsset = (asset: Asset): BrowseMediaItem | null => {
  const assetId = asset.asset_id;
  if (!assetId) return null;

  return {
    type: "media_item",
    id: `media:${assetId}`,
    mediaItemId: assetId,
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

export const findBrowseItemById = (items: BrowseItem[], itemId: string): BrowseItem | undefined =>
  items.find((item) => item.id === itemId);

export const findBrowseItemIndexByAssetId = (items: BrowseItem[], assetId: string): number =>
  items.findIndex((item) => {
    if (item.type === "media_item") {
      return item.asset.asset_id === assetId;
    }

    if (getBrowseItemAsset(item).asset_id === assetId) {
      return true;
    }

    return (
      item.assets.some((a) => a.asset_id === assetId) ||
      item.members.some((member) => member.primaryAssetId === assetId)
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
      const memberAssetIds = item.members.map((member) => member.primaryAssetId).filter(Boolean);
      if (memberAssetIds.length > 0) {
        memberAssetIds.forEach(addAssetId);
        return;
      }
    }

    addAssetId(getBrowseItemAssetId(item));
  });

  return resolved;
};

const stackKindFromPreview = (stack?: StackPreview | null): BrowseStackKind =>
  stack?.stack_kind === "burst" ? "burst" : "manual";

const memberRefFromAsset = (asset: Asset): BrowseStackMemberRef[] =>
  asset.asset_id ? [{ mediaItemId: asset.asset_id, primaryAssetId: asset.asset_id }] : [];

export const createBrowseGroupsFromAssetGroups = (groups?: AssetGroup[]): BrowseGroup[] => {
  if (!groups || groups.length === 0) return [];

  const stackItemsById = new Map<string, BrowseItem>();
  const stackGroupIndexById = new Map<string, number>();
  const browseGroups: BrowseGroup[] = [];

  groups.forEach((group) => {
    const items: BrowseItem[] = [];

    group.assets.forEach((asset) => {
      if (!asset.asset_id) return;

      if (!isStackAsset(asset)) {
        const mediaItem = toMediaItemFromAsset(asset);
        if (mediaItem) items.push(mediaItem);
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
          stackKind: stackKindFromPreview(asset.stack),
          representative: asset,
          assets: [asset],
          members: memberRefFromAsset(asset),
          matchedMembers: memberRefFromAsset(asset),
        };
        stackItemsById.set(stackId, stackItem);
        stackGroupIndexById.set(stackId, browseGroups.length);
        items.push(stackItem);
        return;
      }

      existingItem.assets = [...existingItem.assets, asset];
      if (asset.asset_id) {
        existingItem.members = [...existingItem.members, ...memberRefFromAsset(asset)];
        existingItem.matchedMembers = [
          ...existingItem.matchedMembers,
          ...memberRefFromAsset(asset),
        ];
      }
      const nextRepresentative = preferRepresentative(existingItem.representative, asset);
      if (nextRepresentative === existingItem.representative) {
        return;
      }

      existingItem.representative = nextRepresentative;
      const existingGroupIndex = stackGroupIndexById.get(stackId);
      if (existingGroupIndex === undefined) return;

      const existingGroup = browseGroups[existingGroupIndex];
      if (!existingGroup) return;

      existingGroup.items = existingGroup.items.filter((item) => item.id !== existingItem.id);
      items.push(existingItem);
      stackGroupIndexById.set(stackId, browseGroups.length);
    });

    if (items.length === 0) return;
    browseGroups.push({ key: group.key, items });
  });

  return browseGroups.filter((group) => group.items.length > 0);
};

export const createBrowseGroupsFromAssets = (assets?: Asset[], key = "flat:all"): BrowseGroup[] =>
  createBrowseGroupsFromAssetGroups(assets && assets.length > 0 ? [{ key, assets }] : []);

const toStackMemberRefs = (members?: BrowseStackMemberDTO[] | null): BrowseStackMemberRef[] =>
  (members ?? []).flatMap((member) =>
    member.media_item_id && member.primary_asset_id
      ? [{ mediaItemId: member.media_item_id, primaryAssetId: member.primary_asset_id }]
      : [],
  );

const toCompositionFacts = (
  composition?: MediaCompositionDTO | null,
): MediaCompositionFacts | undefined =>
  composition
    ? {
        componentCount: composition.component_count ?? 1,
        hasRaw: composition.has_raw ?? false,
        hasJpeg: composition.has_jpeg ?? false,
        hasEdited: composition.has_edited ?? false,
        hasLiveMotion: composition.has_live_motion ?? false,
      }
    : undefined;

export const createBrowseItemsFromBrowseItemDTOs = (
  dtoItems?: BrowseItemDTO[] | null,
): BrowseItem[] => {
  if (!dtoItems || dtoItems.length === 0) return [];

  const items: BrowseItem[] = [];

  dtoItems.forEach((item) => {
    if (item.type === "stack" && item.stack) {
      const stackId = item.stack.stack_id;
      const cover = item.stack.cover;
      const primaryAsset = cover?.primary_asset as Asset | undefined;
      if (!stackId || !primaryAsset?.asset_id) return;

      // Graft the cover's stack preview onto the asset so thumbnail overlays
      // (stack size badge) keep working off `asset.stack`.
      const representative: Asset = { ...primaryAsset, stack: cover?.stack ?? primaryAsset.stack };

      items.push({
        type: "stack",
        id: `stack:${stackId}`,
        stackId,
        stackKind: item.stack.stack_kind === "burst" ? "burst" : "manual",
        representative,
        assets: [representative],
        members: toStackMemberRefs(item.stack.members),
        matchedMembers: toStackMemberRefs(item.stack.matched_members),
        bestTsMs: item.best_ts_ms ?? undefined,
      });
      return;
    }

    if (item.type === "media_item" && item.media_item?.primary_asset?.asset_id) {
      const media = item.media_item;
      const primaryAsset = media.primary_asset as Asset;
      const mediaItemId = media.media_item_id ?? primaryAsset.asset_id ?? "";
      if (!mediaItemId) return;

      items.push({
        type: "media_item",
        id: `media:${mediaItemId}`,
        mediaItemId,
        asset: { ...primaryAsset, stack: media.stack ?? primaryAsset.stack },
        composition: toCompositionFacts(media.composition),
        bestTsMs: item.best_ts_ms ?? undefined,
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

/**
 * Date grouping runs on browse items, not on physical assets: two rows can share
 * a representative asset (a media item and the stack it covers), so a round-trip
 * through `asset_id` would drop or duplicate rows. Grouping keys come from the
 * representative's date, but row identity stays `BrowseItem.id`.
 */
export const groupBrowseItemsBySort = (
  items: BrowseItem[],
  sortBy: SortByType,
  now: Date = new Date(),
): BrowseGroup[] => {
  if (items.length === 0) return [];

  const groups: BrowseGroup[] = [];
  items.forEach((item) => {
    const key = getAssetGroupKey(getBrowseItemAsset(item), sortBy, now);
    const last = groups[groups.length - 1];
    if (last && last.key === key) {
      last.items = [...last.items, item];
      return;
    }

    groups.push({ key, items: [item] });
  });

  return groups;
};

export const mergeAdjacentBrowseGroups = (...groupCollections: BrowseGroup[][]): BrowseGroup[] => {
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
