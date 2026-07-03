import { Share2 } from "lucide-react";
import type {
  AssetsBulkActionContext,
  AssetsBulkActionItem,
} from "@/features/assets/components/shared/bulkActions";

/**
 * "Share selected" bulk action, reusable across every gallery that already
 * supports multi-select (Assets, Liked, Album, Person, Utility classifier).
 * Unlike other bulk actions, onRun doesn't mutate anything itself — it hands
 * the selected asset IDs to the caller's CreateShareLinkModal, since share
 * creation needs a form (title/expiry/download policy).
 */
export function createShareSelectedBulkAction(
  label: string,
  onOpen: (assetIds: string[]) => void,
): AssetsBulkActionItem {
  return {
    id: "share-selected",
    label,
    icon: <Share2 size={16} />,
    onRun: (context: AssetsBulkActionContext) => {
      onOpen(context.selectedAssetIds);
    },
  };
}
