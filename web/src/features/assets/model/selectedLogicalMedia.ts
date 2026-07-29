import type { SelectedLogicalMedia } from "@/lib/assets/bulkActions";
import type { BrowseItem } from "../types";

export function toSelectedLogicalMedia(item: BrowseItem): SelectedLogicalMedia {
  if (item.type === "media_item") {
    return {
      browse_item_id: item.id,
      media_item_ids: [item.mediaItemId],
      representative_asset_ids: item.asset.asset_id ? [item.asset.asset_id] : [],
      complete: true,
    };
  }
  const mediaItemIDs = [...new Set(item.members.map((member) => member.mediaItemId))];
  const representativeAssetIDs = [
    ...new Set(item.members.map((member) => member.primaryAssetId).filter(Boolean)),
  ];
  return {
    browse_item_id: item.id,
    media_item_ids: mediaItemIDs,
    representative_asset_ids: representativeAssetIDs,
    complete: mediaItemIDs.length > 0 && item.assets.length >= mediaItemIDs.length,
  };
}
