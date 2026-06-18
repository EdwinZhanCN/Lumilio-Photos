import { useParams } from "react-router-dom";
import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { AlbumIcon, Bird, RefreshCcw } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album } from "@/lib/albums/types";
import type { components } from "@/lib/http-commons/schema";
import { useWorkingRepository } from "@/features/settings";
import { useI18n } from "@/lib/i18n.tsx";

type RebuildAlbumBioClipResponse =
  components["schemas"]["dto.RebuildAlbumBioClipResponseDTO"];

const AlbumAssetsContent = () => {
  const { t, i18n } = useI18n();
  const queryClient = useQueryClient();
  const { albumId } = useParams<{ albumId: string }>();
  const { scopedRepositoryId } = useWorkingRepository();
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
  const isBioAlbum = album?.album_type === "bio";
  const rebuildBioClipMutation = $api.useMutation(
    "post",
    "/api/v1/albums/{id}/bioclip/rebuild",
  );

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

  const hero = (
    <div className="px-4 py-4">
      <div className="flex items-baseline gap-4">
        {isAlbumLoading && !album ? (
          <div className="h-10 w-64 animate-pulse rounded-lg bg-base-300" />
        ) : (
          <>
            <h1 className="text-4xl font-black tracking-tight text-primary">
              {album?.album_name || t("collections.untitled")}
            </h1>
            <span className="badge badge-ghost font-mono text-xs opacity-50">
              {t("collections.albumDetails.albumCode", { id: albumId })}
            </span>
            {isBioAlbum && (
              <span className="badge badge-primary gap-1.5">
                <Bird className="size-3.5" />
                {t("collections.albumDetails.bioClip.badge")}
              </span>
            )}
          </>
        )}
      </div>

      {album?.description && (
        <p className="mt-3 max-w-3xl leading-relaxed text-base-content/70 line-clamp-2">
          {album.description}
        </p>
      )}

      <div className="mt-6 flex items-center gap-6 text-xs opacity-40">
        <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
          <span className="text-[8px] text-primary">●</span>
          {isAlbumLoading && !album ? (
            <div className="h-3 w-16 animate-pulse rounded bg-base-300" />
          ) : (
            <span>
              {t("collections.albumDetails.itemsCount", {
                count: album?.asset_count || 0,
              })}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
          <span className="text-[8px] text-primary">●</span>
          {isAlbumLoading && !album ? (
            <div className="h-3 w-24 animate-pulse rounded bg-base-300" />
          ) : (
            <span>
              {t("collections.albumDetails.createdAtLabel")}{" "}
              {album?.created_at
                ? new Date(album.created_at).toLocaleDateString(
                    i18n.resolvedLanguage || i18n.language,
                  )
                : ""}
            </span>
          )}
        </div>

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
      </div>

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
    <AssetsGalleryPage
      title={album?.album_name || t("collections.albumDetails.fallbackName")}
      icon={<AlbumIcon className="h-6 w-6 text-primary" />}
      baseFilter={{ album_id: albumIdNumber }}
      viewKey={`album:${albumId}`}
      hero={hero}
    />
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
