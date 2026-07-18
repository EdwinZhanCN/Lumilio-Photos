import { useCallback, useMemo, useState } from "react";
import { Layers2 } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { useAlbumOptions } from "@/lib/albums/useAlbumOptions";
import {
  isBulkActionHidden,
  resolveAssetsBulkActions,
  type AssetsBulkActionContext,
  type AssetsBulkActionId,
  type AssetsBulkActionItem,
} from "@/lib/assets/bulkActions";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useI18n } from "@/lib/i18n";
import { useBulkAssetActions } from "../bulk-actions/useBulkAssetActions";
import { useAssetSelection } from "../selection/useAssetSelection";
import { useStackActions } from "../../../api/useStackActions";
import {
  getBrowseItemAsset,
  resolveBrowseSelectedAssetIds,
  resolveSelectedBrowseItems,
} from "../../../model/browseItems";
import type { TagPickerItem } from "../../../components/TagPickerMenu";
import type { AssetsPageHeaderProps, ConfirmableBulkAction } from "./types";

type TagSuggestion = components["schemas"]["dto.TagDTO"];

type BulkActionOptions = Pick<
  AssetsPageHeaderProps,
  "browseItems" | "bulkActions" | "hiddenBulkActions"
>;

export function useAssetsPageHeaderBulkActions({
  browseItems,
  bulkActions,
  hiddenBulkActions,
}: BulkActionOptions) {
  const { t } = useI18n();
  const selection = useAssetSelection();
  const showMessage = useMessage();
  const { createStack, isCreatingStack } = useStackActions();

  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [confirmableBulkAction, setConfirmableBulkAction] = useState<ConfirmableBulkAction | null>(
    null,
  );
  const [confirmableCustomAction, setConfirmableCustomAction] =
    useState<AssetsBulkActionItem | null>(null);
  const [isRunningCustomAction, setIsRunningCustomAction] = useState(false);
  const [isAlbumModalOpen, setIsAlbumModalOpen] = useState(false);
  const [isAddingToAlbum, setIsAddingToAlbum] = useState(false);
  const [isTagsModalOpen, setIsTagsModalOpen] = useState(false);
  const [isAddingTags, setIsAddingTags] = useState(false);
  const [tagQuery, setTagQuery] = useState("");
  const [pendingTags, setPendingTags] = useState<TagPickerItem[]>([]);

  const albumOptionsQuery = useAlbumOptions(isAlbumModalOpen);
  const albums = albumOptionsQuery.data?.albums ?? [];
  const isLoadingAlbums = albumOptionsQuery.isPending;
  const tagSuggestionsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/tags",
    { params: { query: { q: tagQuery, limit: 20 } } },
    { enabled: isTagsModalOpen, staleTime: 30_000 },
  );

  const effectiveBrowseItems = browseItems ?? [];
  const selectedBrowseItems = useMemo(() => {
    if (!effectiveBrowseItems || effectiveBrowseItems.length === 0) return [];
    return resolveSelectedBrowseItems(selection.selectedIds, effectiveBrowseItems);
  }, [effectiveBrowseItems, selection.selectedIds]);

  const resolvedSelectedAssetIds = useMemo(
    () =>
      resolveBrowseSelectedAssetIds(selection.selectedIds, effectiveBrowseItems, {
        stackMode: "whole-stack",
      }),
    [effectiveBrowseItems, selection.selectedIds],
  );

  const bulkOps = useBulkAssetActions(resolvedSelectedAssetIds);
  const affectedAssetCount = resolvedSelectedAssetIds.length;
  const selectedItemCount = selectedBrowseItems.length || selection.selectedCount;
  const showAffectedAssetCount = affectedAssetCount > 0 && affectedAssetCount !== selectedItemCount;

  const selectedAssets = useMemo(() => {
    if (!selection.enabled || selection.selectedCount === 0) return [];
    return selectedBrowseItems.flatMap((item) =>
      item.type === "stack" ? item.assets : [getBrowseItemAsset(item)],
    );
  }, [selection.enabled, selection.selectedCount, selectedBrowseItems]);

  const bulkActionContext = useMemo<AssetsBulkActionContext>(
    () => ({
      selectedItemCount,
      affectedAssetCount,
      selectedAssetIds: resolvedSelectedAssetIds,
      selectedAssets,
      clearSelection: selection.clear,
    }),
    [
      affectedAssetCount,
      resolvedSelectedAssetIds,
      selectedAssets,
      selectedItemCount,
      selection.clear,
    ],
  );

  const customBulkActions = useMemo(
    () => resolveAssetsBulkActions(bulkActions, bulkActionContext),
    [bulkActions, bulkActionContext],
  );

  const isDefaultActionHidden = useCallback(
    (actionId: AssetsBulkActionId) => isBulkActionHidden(actionId, hiddenBulkActions),
    [hiddenBulkActions],
  );

  const stackSelectedBulkAction = useMemo<AssetsBulkActionItem>(
    () => ({
      id: "stack-selected",
      label: t("assets.assetsPageHeader.actions.stackSelected", {
        defaultValue: "Stack selected",
      }),
      icon: <Layers2 size={16} />,
      tone: "info",
      disabled: affectedAssetCount < 2 || isCreatingStack,
      requiresConfirmation: true,
      confirmationTitle: t("assets.assetsPageHeader.stackConfirm.title", {
        defaultValue: "Stack selected assets?",
      }),
      confirmationMessage: t("assets.assetsPageHeader.stackConfirm.message", {
        count: affectedAssetCount,
        defaultValue: "{{count}} selected assets will be grouped into one stack.",
      }),
      onRun: async (context) => {
        if (context.selectedAssetIds.length < 2) return;

        try {
          await createStack(context.selectedAssetIds);
          context.clearSelection();
          showMessage(
            "success",
            t("assets.assetsPageHeader.messages.stackSuccess", {
              count: context.affectedAssetCount,
            }),
          );
        } catch (error) {
          console.error("Failed to stack selected assets:", error);
          showMessage("error", t("assets.assetsPageHeader.messages.stackError"));
          throw error;
        }
      },
    }),
    [affectedAssetCount, createStack, isCreatingStack, showMessage, t],
  );

  const visibleCustomBulkActions = useMemo(
    () => customBulkActions.filter((action) => !isBulkActionHidden(action.id, hiddenBulkActions)),
    [customBulkActions, hiddenBulkActions],
  );

  const hasBulkActionItems =
    !isDefaultActionHidden("set-rating") ||
    !isDefaultActionHidden("set-liked") ||
    !isDefaultActionHidden("stack-selected") ||
    visibleCustomBulkActions.length > 0 ||
    !isDefaultActionHidden("add-tags") ||
    !isDefaultActionHidden("add-to-album") ||
    !isDefaultActionHidden("download") ||
    !isDefaultActionHidden("delete-assets");

  const handleToggleSelection = useCallback(() => {
    selection.setEnabled(!selection.enabled);
  }, [selection]);

  const handleDeleteClick = () => {
    setIsDeleteConfirmOpen(true);
  };

  const confirmDelete = async () => {
    try {
      await bulkOps.bulkDelete();
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.deleteSuccess", {
          count: affectedAssetCount,
        }),
      );
    } catch {
      showMessage("error", t("assets.assetsPageHeader.messages.deleteError"));
    } finally {
      setIsDeleteConfirmOpen(false);
    }
  };

  const handleDownloadAll = async () => {
    try {
      await bulkOps.bulkDownload(selectedAssets);
      showMessage("success", t("assets.assetsPageHeader.messages.downloadStart"));
    } catch {
      showMessage("error", t("assets.assetsPageHeader.messages.downloadError"));
    }
  };

  const confirmBulkAction = async () => {
    if (!confirmableBulkAction) return;

    try {
      if (confirmableBulkAction.type === "rating") {
        await bulkOps.bulkUpdateRating(confirmableBulkAction.rating);
      } else {
        await bulkOps.bulkSetLike(confirmableBulkAction.liked);
      }
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.bulkActionSuccess", {
          count: affectedAssetCount,
          defaultValue: "Updated {{count}} assets.",
        }),
      );
    } catch {
      showMessage(
        "error",
        t("assets.assetsPageHeader.messages.bulkActionError", {
          defaultValue: "Failed to update selected assets.",
        }),
      );
    } finally {
      setConfirmableBulkAction(null);
    }
  };

  const handleAddToAlbumClick = () => {
    setIsAlbumModalOpen(true);
  };

  const handleSelectAlbum = async (albumId: number) => {
    setIsAddingToAlbum(true);
    try {
      await bulkOps.bulkAddToAlbum(albumId);
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.addToAlbumSuccess", {
          count: affectedAssetCount,
        }),
      );
      setIsAlbumModalOpen(false);
      selection.clear();
    } catch {
      showMessage("error", t("assets.assetsPageHeader.messages.addToAlbumError"));
    } finally {
      setIsAddingToAlbum(false);
    }
  };

  const closeTagsModal = () => {
    setIsTagsModalOpen(false);
    setTagQuery("");
    setPendingTags([]);
  };

  const handleAddTagsClick = () => {
    setTagQuery("");
    setPendingTags([]);
    setIsTagsModalOpen(true);
  };

  const pendingTagNames = useMemo(
    () => new Set(pendingTags.map((tag) => tag.name.toLowerCase())),
    [pendingTags],
  );

  const tagSuggestionItems = useMemo<TagPickerItem[]>(() => {
    const raw: TagSuggestion[] = tagSuggestionsQuery.data?.tags ?? [];
    return raw
      .filter(
        (tag) => Boolean(tag.tag_name) && !pendingTagNames.has((tag.tag_name ?? "").toLowerCase()),
      )
      .map((tag) => ({
        id: tag.tag_id ?? tag.tag_name!,
        name: tag.tag_name!,
      }));
  }, [pendingTagNames, tagSuggestionsQuery.data?.tags]);

  const trimmedTagQuery = tagQuery.trim();
  const tagExactExists =
    trimmedTagQuery.length > 0 &&
    (pendingTagNames.has(trimmedTagQuery.toLowerCase()) ||
      tagSuggestionItems.some((tag) => tag.name.toLowerCase() === trimmedTagQuery.toLowerCase()));
  const showCreateTag = trimmedTagQuery.length > 0 && !tagExactExists;

  const addPendingTag = (item: TagPickerItem) => {
    const name = item.name.trim();
    if (!name) return;
    setPendingTags((prev) => {
      if (prev.some((tag) => tag.name.toLowerCase() === name.toLowerCase())) return prev;
      return [...prev, { id: item.id, name }];
    });
    setTagQuery("");
  };

  const removePendingTag = (item: TagPickerItem) => {
    setPendingTags((prev) =>
      prev.filter((tag) => tag.name.toLowerCase() !== item.name.toLowerCase()),
    );
  };

  const handleCreatePendingTag = () => {
    if (!trimmedTagQuery) return;
    addPendingTag({ id: trimmedTagQuery, name: trimmedTagQuery });
  };

  const handleApplyTags = async () => {
    if (pendingTags.length === 0) return;
    setIsAddingTags(true);
    try {
      await bulkOps.bulkAddTags(pendingTags.map((tag) => tag.name));
      showMessage(
        "success",
        t("assets.assetsPageHeader.messages.addTagsSuccess", {
          count: affectedAssetCount,
          defaultValue: "Added tags to {{count}} assets.",
        }),
      );
      closeTagsModal();
      selection.clear();
    } catch {
      showMessage(
        "error",
        t("assets.assetsPageHeader.messages.addTagsError", {
          defaultValue: "Failed to add tags to selected assets.",
        }),
      );
    } finally {
      setIsAddingTags(false);
    }
  };

  const executeCustomBulkAction = useCallback(
    async (action: AssetsBulkActionItem) => {
      if (action.disabled) return;

      setIsRunningCustomAction(true);
      try {
        await action.onRun(bulkActionContext);
      } finally {
        setIsRunningCustomAction(false);
        setConfirmableCustomAction(null);
      }
    },
    [bulkActionContext],
  );

  const handleCustomBulkActionClick = useCallback(
    (action: AssetsBulkActionItem) => {
      if (action.disabled) return;
      if (action.requiresConfirmation) {
        setConfirmableCustomAction(action);
        return;
      }
      void executeCustomBulkAction(action);
    },
    [executeCustomBulkAction],
  );

  const renderAffectedAssetHint = () =>
    showAffectedAssetCount
      ? t("assets.assetsPageHeader.actions.affectsAssets", {
          selectedCount: selectedItemCount,
          assetCount: affectedAssetCount,
          defaultValue: "{{selectedCount}} selected items will affect {{assetCount}} assets.",
        })
      : t("assets.assetsPageHeader.actions.affectsSelected", {
          count: selectedItemCount,
          defaultValue: "{{count}} selected items.",
        });

  return {
    selection,
    selectedItemCount,
    showAffectedAssetCount,
    visibleCustomBulkActions,
    stackSelectedBulkAction,
    hasBulkActionItems,
    isDefaultActionHidden,
    handleToggleSelection,
    handleDeleteClick,
    handleDownloadAll,
    handleAddToAlbumClick,
    handleAddTagsClick,
    handleCustomBulkActionClick,
    confirmableBulkAction,
    setConfirmableBulkAction,
    confirmBulkAction,
    confirmableCustomAction,
    setConfirmableCustomAction,
    isRunningCustomAction,
    executeCustomBulkAction,
    isDeleteConfirmOpen,
    setIsDeleteConfirmOpen,
    confirmDelete,
    isAlbumModalOpen,
    setIsAlbumModalOpen,
    albums,
    isLoadingAlbums,
    isAddingToAlbum,
    handleSelectAlbum,
    isTagsModalOpen,
    closeTagsModal,
    tagQuery,
    setTagQuery,
    pendingTags,
    tagSuggestionItems,
    showCreateTag,
    trimmedTagQuery,
    addPendingTag,
    removePendingTag,
    handleCreatePendingTag,
    tagSuggestionsQuery,
    isAddingTags,
    handleApplyTags,
    renderAffectedAssetHint,
  };
}

export type AssetsPageHeaderBulkActions = ReturnType<typeof useAssetsPageHeaderBulkActions>;
