import { useParams } from "react-router-dom";
import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AlbumIcon, Bird, FolderMinus, Pencil, RefreshCcw } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album } from "@/lib/albums/types";
import type { components } from "@/lib/http-commons/schema";
import { useWorkingRepository } from "@/features/settings";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { CollectionTitle, MetaStat, MetaStatRow } from "@/components/collection";
import AlbumFormModal from "../components/AlbumFormModal";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import type {
  AssetsBulkActionContext,
  AssetsBulkActionItem,
} from "@/features/assets/components/shared/bulkActions";

type RebuildAlbumBioClipResponse =
  components["schemas"]["dto.RebuildAlbumBioClipResponseDTO"];

const AlbumAssetsContent = () => {
  const { t, i18n } = useI18n();
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { albumId } = useParams<{ albumId: string }>();
  const { scopedRepositoryId } = useWorkingRepository();
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [bioClipFeedback, setBioClipFeedback] = useState<{
    tone: "success" | "error";
    message: string;
  } | null>(null);
  const albumIdNumber = albumId ? Number(albumId) : 0;

  // Fetch album metadata for the hero banner.
  const albumQuery = $api.useQuery(
    "get",
    "/api/v1/albums/{id}",
    {
      params: {
        path: { id: albumIdNumber },
        query: scopedRepositoryId ? { repository_id: scopedRepositoryId } : {},
      },
    },
    { enabled: !!albumId },
  );
  const album = albumQuery.data as Album | undefined;
  const isAlbumLoading = albumQuery.isLoading;
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.albums", "Albums"), to: "/collections/albums" },
    { label: album?.album_name || t("collections.albumDetails.fallbackName", "Album") },
  ]);
  const isBioAlbum = album?.album_type === "bio";
  const rebuildBioClipMutation = $api.useMutation(
    "post",
    "/api/v1/albums/{id}/bioclip/rebuild",
  );
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
      const responseData = response as RebuildAlbumBioClipResponse | undefined;
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
      {
        id: "remove-from-current-album",
        label: t("collections.albumDetails.bulkActions.removeFromAlbum.label", {
          defaultValue: "Remove from this album",
        }),
        icon: <FolderMinus size={16} />,
        tone: "danger",
        requiresConfirmation: true,
        confirmationTitle: t(
          "collections.albumDetails.bulkActions.removeFromAlbum.confirmTitle",
          { defaultValue: "Remove selected items from this album?" },
        ),
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
              t(
                "collections.albumDetails.bulkActions.removeFromAlbum.success",
                { count: context.affectedAssetCount },
              ),
            );
          } catch (error) {
            console.error("Failed to remove selected assets from album:", error);
            showMessage(
              "error",
              t("collections.albumDetails.bulkActions.removeFromAlbum.error"),
            );
            throw error;
          }
        },
      },
    ],
    [
      albumIdNumber,
      invalidateAlbumAssets,
      removeAssetFromAlbumMutation,
      showMessage,
      t,
    ],
  );

  const hero = (
    <div className="px-4 py-4">
      <CollectionTitle
        loading={isAlbumLoading && !album}
        title={album?.album_name || t("collections.untitled")}
        code={t("collections.albumDetails.albumCode", { id: albumId })}
      >
        {isBioAlbum && (
          <span className="badge badge-primary gap-1.5">
            <Bird className="size-3.5" />
            {t("collections.albumDetails.bioClip.badge")}
          </span>
        )}
        <button
          type="button"
          className="btn btn-ghost btn-sm gap-1.5 rounded-full"
          onClick={() => setIsEditOpen(true)}
        >
          <Pencil className="size-3.5" />
          {t("common.edit", "Edit")}
        </button>
      </CollectionTitle>

      {album?.description && (
        <p className="mt-3 max-w-3xl leading-relaxed text-base-content/70 line-clamp-2">
          {album.description}
        </p>
      )}

      <MetaStatRow className="mt-6">
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
      </MetaStatRow>

      {bioClipFeedback && (
        <div
          className={`alert mt-4 max-w-xl py-2 text-sm ${
            bioClipFeedback.tone === "success" ? "alert-success" : "alert-error"
          }`}
        >
          {bioClipFeedback.message}
        </div>
      )}
    </div>
  );

  return (
    <>
      <AssetsGalleryPage
        title={album?.album_name || t("collections.albumDetails.fallbackName")}
        icon={<AlbumIcon className="h-6 w-6 text-primary" />}
        baseFilter={{ album_id: albumIdNumber }}
        viewKey={`album:${albumId}`}
        hero={hero}
        bulkActions={bulkActions}
        hiddenBulkActions={["delete-assets"]}
      />
      <AlbumFormModal
        open={isEditOpen}
        mode="edit"
        album={album}
        onClose={() => setIsEditOpen(false)}
      />
    </>
  );
};

const AlbumDetails = () => {
  const { albumId } = useParams<{ albumId: string }>();

  return (
    <WorkerProvider>
      <AssetsProvider
        key={`album:${albumId}`}
        scopeId={`album:${albumId}`}
        persist={false}
        basePath={`/collections/${albumId}`}
      >
        <AlbumAssetsContent />
      </AssetsProvider>
    </WorkerProvider>
  );
};

export default AlbumDetails;
