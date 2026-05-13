import type { Asset } from "@/lib/assets/types";
import type { AssetGroup } from "@/features/assets/types/assets.type";

const isCollapsibleStackAsset = (asset: Asset): boolean =>
  Boolean(
    asset.stack?.stack_id &&
      asset.stack?.stack_size &&
      asset.stack.stack_size > 1,
  );

const shouldPreferStackRepresentative = (
  current: Asset,
  candidate: Asset,
): boolean => Boolean(!current.stack?.stack_cover && candidate.stack?.stack_cover);

export const collapseStackedAssets = (assets: Asset[]): Asset[] => {
  const collapsed: Asset[] = [];
  const stackIndexById = new Map<string, number>();

  assets.forEach((asset) => {
    const stackId = asset.stack?.stack_id;

    if (!isCollapsibleStackAsset(asset) || !stackId) {
      collapsed.push(asset);
      return;
    }

    const existingIndex = stackIndexById.get(stackId);
    if (existingIndex === undefined) {
      stackIndexById.set(stackId, collapsed.length);
      collapsed.push(asset);
      return;
    }

    if (shouldPreferStackRepresentative(collapsed[existingIndex], asset)) {
      collapsed[existingIndex] = asset;
    }
  });

  return collapsed;
};

export const collapseStackedAssetGroups = (
  groups: AssetGroup[],
): AssetGroup[] =>
  groups.map((group) => ({
    ...group,
    assets: collapseStackedAssets(group.assets),
  }));
