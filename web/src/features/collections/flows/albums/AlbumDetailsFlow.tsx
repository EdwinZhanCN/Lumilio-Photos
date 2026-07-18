import { useParams } from "react-router-dom";
import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AssetBrowser, AssetBrowserScope } from "@/features/assets";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AlbumIcon, Bird, FolderMinus, RefreshCcw, Share2 } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { CollectionHero, MetaStat } from "@/components/collection";
import AlbumFormModal from "./components/AlbumFormModal";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/features/notifications";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";
import {
  CreateShareLinkModal,
  createShareSelectedBulkAction,
  type ShareSourceKind,
} from "@/features/share";

const AlbumAssetsContent = () => {
  const { t, i18n } = useI18n();
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { albumId } = useParams<{ albumId: string }>();
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [shareRequest, setShareRequest] = useState<{
    sourceKind: ShareSourceKind;
    assetIds?: string[];
    sourceRef?: string;
  } | null>(null);
  const [bioClipFeedback, setBioClipFeedback] = useState<{
    tone: "success" | "error";
    message: string;
  } | null>(null);
  const albumIdNumber = albumId ? Number(albumId) : 0;

  // Fetch album metadata for the hero banner. An album intentionally spans
  // repositories, so the browse scope never filters its detail or members.
  const albumQuery = $api.useQuery(
    "get",
    "/api/v1/albums/{id}",
    {
      params: {
        path: { id: albumIdNumber },
      },
    },
    { enabled: !!albumId },
  );
  const album = albumQuery.data;
  const isAlbumLoading = albumQuery.isLoading;
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.albums", "Albums"), to: "/collections/albums" },
    { label: album?.album_name || t("collections.albumDetails.fallbackName", "Album") },
  ]);
  const isBioAlbum = album?.album_type === "bio";
  const rebuildBioClipMutation = $api.useMutation("post", "/api/v1/albums/{id}/bioclip/rebuild");
  const removeAssetFromAlbumMutation = $api.useMutation(
    "delete",
    "/api/v1/albums/{id}/assets/{assetId}",
  );

  const invalidateAlbumAssets = useCallback(async () => {
    await queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (!Array.isArray(key)) return false;

        const path = key[1];
        return (
          path === "/api/v1/assets/list" ||
          path === "/api/v1/assets/search" ||
          path === "/api/v1/albums/{id}"
        );
      },
    });
  }, [queryClient]);

  const handleRebuildBioClip = useCallback(async () => {
    if (!albumIdNumber || !isBioAlbum) return;

    setBioClipFeedback(null);
    try {
      const response = await rebuildBioClipMutation.mutateAsync({
        params: { path: { id: albumIdNumber } },
        body: {},
      });
      const responseData = response;
      const queuedAssets = responseData?.queued_assets ?? 0;

      await queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/stats"],
      });
      setBioClipFeedback({
        tone: "success",
        message: t("collections.albumDetails.bioClip.success", {
          count: queuedAssets,
        }),
      });
    } catch (error) {
      console.error("Failed to rebuild BioCLIP for album:", error);
      setBioClipFeedback({
        tone: "error",
        message: t("collections.albumDetails.bioClip.error"),
      });
    }
  }, [albumIdNumber, isBioAlbum, queryClient, rebuildBioClipMutation, t]);

  const bulkActions = useCallback(
    (context: AssetsBulkActionContext): AssetsBulkActionItem[] => [
      createShareSelectedBulkAction(
        t("assets.assetsPageHeader.bulkActions.share.label", "Share"),
        (assetIds) => setShareRequest({ sourceKind: "asset_snapshot", assetIds }),
      ),
      {
        id: "remove-from-current-album",
        label: t("collections.albumDetails.bulkActions.removeFromAlbum.label", {
          defaultValue: "Remove from this album",
        }),
        icon: <FolderMinus size={16} />,
        tone: "danger",
        requiresConfirmation: true,
        confirmationTitle: t("collections.albumDetails.bulkActions.removeFromAlbum.confirmTitle", {
          defaultValue: "Remove selected items from this album?",
        }),
        confirmationMessage: t(
          "collections.albumDetails.bulkActions.removeFromAlbum.confirmMessage",
          {
            count: context.affectedAssetCount,
            defaultValue:
              "{{count}} selected assets will be removed from this album. Original assets remain in the library.",
          },
        ),
        disabled: !albumIdNumber,
        onRun: async (context) => {
          if (!albumIdNumber || context.selectedAssetIds.length === 0) return;

          try {
            await Promise.all(
              context.selectedAssetIds.map((assetId) =>
                removeAssetFromAlbumMutation.mutateAsync({
                  params: { path: { id: albumIdNumber, assetId } },
                  body: {},
                }),
              ),
            );
            context.clearSelection();
            await invalidateAlbumAssets();
            showMessage(
              "success",
              t("collections.albumDetails.bulkActions.removeFromAlbum.success", {
                count: context.affectedAssetCount,
              }),
            );
          } catch (error) {
            console.error("Failed to remove selected assets from album:", error);
            showMessage("error", t("collections.albumDetails.bulkActions.removeFromAlbum.error"));
            throw error;
          }
        },
      },
    ],
    [albumIdNumber, invalidateAlbumAssets, removeAssetFromAlbumMutation, showMessage, t],
  );

  const openShareAlbum = useCallback(() => {
    if (!albumIdNumber) return;
    setShareRequest({ sourceKind: "album", sourceRef: String(albumIdNumber) });
  }, [albumIdNumber]);

  const hero = (
    <CollectionHero
      loading={isAlbumLoading && !album}
      title={album?.album_name || t("collections.untitled")}
      code={t("collections.albumDetails.albumCode", { id: albumId })}
      badges={
        isBioAlbum && (
          <span className="badge badge-primary gap-1.5">
            <Bird className="size-3.5" />
            {t("collections.albumDetails.bioClip.badge")}
          </span>
        )
      }
      description={album?.description}
      actions={
        <button
          type="button"
          className="btn btn-ghost btn-sm gap-1.5 rounded-full"
          onClick={openShareAlbum}
        >
          <Share2 className="size-3.5" />
          {t("collections.albumDetails.shareAction", "Share")}
        </button>
      }
      edit={{
        onOpen: () => setIsEditOpen(true),
        label: t("common.edit", "Edit"),
        modal: (
          <AlbumFormModal
            open={isEditOpen}
            mode="edit"
            album={album}
            onClose={() => setIsEditOpen(false)}
          />
        ),
      }}
      stats={
        <>
          <MetaStat loading={isAlbumLoading && !album}>
            {t("collections.albumDetails.itemsCount", {
              count: album?.asset_count || 0,
            })}
          </MetaStat>
          <MetaStat loading={isAlbumLoading && !album} skeletonWidth="w-24">
            {t("collections.albumDetails.createdAtLabel")}{" "}
            {album?.created_at
              ? new Date(album.created_at).toLocaleDateString(
                  i18n.resolvedLanguage || i18n.language,
                )
              : ""}
          </MetaStat>

          {isBioAlbum && (
            <button
              type="button"
              className="btn btn-primary btn-xs gap-1.5"
              onClick={handleRebuildBioClip}
              disabled={rebuildBioClipMutation.isPending}
            >
              {rebuildBioClipMutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <RefreshCcw className="size-3.5" />
              )}
              {rebuildBioClipMutation.isPending
                ? t("collections.albumDetails.bioClip.running")
                : t("collections.albumDetails.bioClip.action")}
            </button>
          )}
        </>
      }
      footer={
        bioClipFeedback && (
          <div
            className={`alert mt-4 max-w-xl py-2 text-sm ${
              bioClipFeedback.tone === "success" ? "alert-success" : "alert-error"
            }`}
          >
            {bioClipFeedback.message}
          </div>
        )
      }
    />
  );

  return (
    <>
      <AssetBrowser
        title={album?.album_name || t("collections.albumDetails.fallbackName")}
        icon={<AlbumIcon className="h-6 w-6 text-primary" />}
        constraint={{ album_id: albumIdNumber }}
        viewKey={`album:${albumId}`}
        hero={hero}
        bulkActions={bulkActions}
        hiddenBulkActions={["delete-assets"]}
      />
      <CreateShareLinkModal
        open={shareRequest !== null}
        onClose={() => setShareRequest(null)}
        sourceKind={shareRequest?.sourceKind ?? "asset_snapshot"}
        assetIds={shareRequest?.assetIds}
        sourceRef={shareRequest?.sourceRef}
        defaultTitle={shareRequest?.sourceKind === "album" ? album?.album_name : undefined}
      />
    </>
  );
};

const AlbumDetails = () => {
  const { albumId } = useParams<{ albumId: string }>();

  return (
    <WorkerProvider>
      <AssetBrowserScope
        key={`album:${albumId}`}
        scopeId={`album:${albumId}`}
        basePath={`/collections/${albumId}`}
      >
        <AlbumAssetsContent />
      </AssetBrowserScope>
    </WorkerProvider>
  );
};

export default AlbumDetails;
