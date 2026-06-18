import type { ReactNode } from "react";
import type { Asset } from "@/lib/assets/types";

export type AssetsBulkActionId =
  | "set-rating"
  | "set-liked"
  | "stack-selected"
  | "add-to-album"
  | "download"
  | "delete-assets"
  | "restore-assets"
  | (string & {});

export type AssetsBulkActionTone = "default" | "info" | "danger";

export interface AssetsBulkActionContext {
  selectedItemCount: number;
  affectedAssetCount: number;
  selectedAssetIds: string[];
  selectedAssets: Asset[];
  clearSelection: () => void;
}

export interface AssetsBulkActionItem {
  id: AssetsBulkActionId;
  label: string;
  icon?: ReactNode;
  tone?: AssetsBulkActionTone;
  disabled?: boolean;
  requiresConfirmation?: boolean;
  confirmationTitle?: string;
  confirmationMessage?: string;
  onRun: (context: AssetsBulkActionContext) => Promise<void> | void;
}

export type AssetsBulkActionInput =
  | AssetsBulkActionItem[]
  | ((context: AssetsBulkActionContext) => AssetsBulkActionItem[]);

export const DEFAULT_ASSETS_BULK_ACTION_IDS = [
  "set-rating",
  "set-liked",
  "stack-selected",
  "add-to-album",
  "download",
  "delete-assets",
] as const satisfies readonly AssetsBulkActionId[];

export const resolveAssetsBulkActions = (
  bulkActions: AssetsBulkActionInput | undefined,
  context: AssetsBulkActionContext,
): AssetsBulkActionItem[] => {
  if (!bulkActions) return [];
  return typeof bulkActions === "function" ? bulkActions(context) : bulkActions;
};

export const isBulkActionHidden = (
  actionId: AssetsBulkActionId,
  hiddenBulkActions?: readonly AssetsBulkActionId[],
): boolean => hiddenBulkActions?.includes(actionId) ?? false;
