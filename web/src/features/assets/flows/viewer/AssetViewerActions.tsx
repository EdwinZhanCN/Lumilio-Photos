import { useCallback, useOptimistic, useState, useTransition } from "react";
import { useNavigate } from "react-router-dom";
import { Ellipsis, Heart, Info, Plus, Share, Trash2, X } from "lucide-react";
import { CreateShareLinkModal } from "@/features/share";
import { useAlbumOptions } from "@/lib/albums/useAlbumOptions";
import type { Asset } from "@/lib/http-commons";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { useAssetActions } from "../../api/useAssetActions";
import { AssetExportDialog } from "../export/AssetExportDialog";

interface AssetViewerActionsProps {
  asset: Asset | null;
  deleteTarget: Asset | null;
  showInfo: boolean;
  onToggleInfo: () => void;
  onAssetUpdate: (asset: Asset) => void;
  onAssetDelete: (assetId: string) => void;
}

const DELETE_DIALOG_ID = "asset_viewer_delete_dialog";
const ALBUM_DIALOG_ID = "asset_viewer_album_dialog";

/** Owns all mutations and dialogs launched from the viewer action flower. */
export function AssetViewerActions({
  asset,
  deleteTarget,
  showInfo,
  onToggleInfo,
  onAssetUpdate,
  onAssetDelete,
}: AssetViewerActionsProps) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { toggleLike, deleteAsset } = useAssetActions();
  const [shareAsset, setShareAsset] = useState<Asset | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isAddingToAlbum, setIsAddingToAlbum] = useState(false);
  const albumOptionsQuery = useAlbumOptions();
  const albums = albumOptionsQuery.data?.albums ?? [];
  const addToAlbumMutation = $api.useMutation("post", "/api/v1/albums/{id}/assets/{assetId}");
  const [, startTransition] = useTransition();
  const [optimisticLiked, setOptimisticLiked] = useOptimistic(
    asset?.liked ?? false,
    (_state, liked: boolean) => liked,
  );

  const openDialog = useCallback((id: string) => {
    const dialog = document.getElementById(id) as HTMLDialogElement | null;
    dialog?.showModal();
  }, []);

  const closeDialog = useCallback((id: string) => {
    const dialog = document.getElementById(id) as HTMLDialogElement | null;
    dialog?.close();
  }, []);

  const handleLikeToggle = useCallback(() => {
    if (!asset?.asset_id) return;
    const liked = !optimisticLiked;
    const assetId = asset.asset_id;

    startTransition(async () => {
      setOptimisticLiked(liked);
      try {
        await toggleLike(assetId, liked);
        onAssetUpdate({ ...asset, liked });
      } catch (error) {
        console.error("Failed to update like status:", error);
      }
    });
  }, [asset, onAssetUpdate, optimisticLiked, setOptimisticLiked, startTransition, toggleLike]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget?.asset_id || isDeleting) return;
    const assetId = deleteTarget.asset_id;

    setIsDeleting(true);
    try {
      await deleteAsset(assetId);
      onAssetDelete(assetId);
    } catch (error) {
      console.error("Failed to delete asset:", error);
    } finally {
      setIsDeleting(false);
      closeDialog(DELETE_DIALOG_ID);
    }
  }, [closeDialog, deleteAsset, deleteTarget?.asset_id, isDeleting, onAssetDelete]);

  const handleSelectAlbum = useCallback(
    async (albumId: number) => {
      if (!asset?.asset_id) return;
      setIsAddingToAlbum(true);
      try {
        await addToAlbumMutation.mutateAsync({
          params: { path: { id: albumId, assetId: asset.asset_id } },
          body: {},
        });
        closeDialog(ALBUM_DIALOG_ID);
      } catch {
        // Keep the picker open so the user can retry.
      } finally {
        setIsAddingToAlbum(false);
      }
    },
    [addToAlbumMutation, asset?.asset_id, closeDialog],
  );

  return (
    <>
      <AssetExportDialog
        asset={asset ?? undefined}
        onOpenStudio={(selectedAsset) => {
          void navigate(`/studio?assetId=${selectedAsset.asset_id}`);
        }}
        onAddToAlbum={() => openDialog(ALBUM_DIALOG_ID)}
        onShare={setShareAsset}
      />

      <CreateShareLinkModal
        open={shareAsset !== null}
        onClose={() => setShareAsset(null)}
        sourceKind="asset_snapshot"
        assetIds={shareAsset?.asset_id ? [shareAsset.asset_id] : undefined}
        defaultTitle={shareAsset?.original_filename ?? undefined}
      />

      <dialog id={DELETE_DIALOG_ID} className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg text-error">{t("delete.confirmTitle")}</h3>
          <p className="py-4">
            {t("delete.confirmMessage", {
              filename: deleteTarget?.original_filename || t("delete.thisAsset"),
            })}
          </p>
          <p className="text-sm text-base-content/70 mb-4">{t("delete.softDeleteNote")}</p>
          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost mr-2" disabled={isDeleting}>
                {t("common.cancel")}
              </button>
              <button
                type="button"
                className={`btn btn-error ${isDeleting ? "loading" : ""}`}
                onClick={() => void handleDelete()}
                disabled={isDeleting}
              >
                {isDeleting ? "" : <Trash2 className="w-4 h-4 mr-2" />}
                {isDeleting ? t("delete.deleting") : t("delete.confirm")}
              </button>
            </form>
          </div>
        </div>
      </dialog>

      <dialog id={ALBUM_DIALOG_ID} className="modal">
        <div className="modal-box">
          <form method="dialog">
            <button
              className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2"
              aria-label={t("common.close")}
            >
              <X />
            </button>
          </form>
          <h3 className="font-bold text-lg mb-4">
            {t("assets.assetsPageHeader.addToAlbumModal.title", {
              defaultValue: "Add to Album",
            })}
          </h3>

          {albumOptionsQuery.isPending ? (
            <div className="flex justify-center py-12">
              <span className="loading loading-spinner loading-md text-primary" />
            </div>
          ) : albums.length > 0 ? (
            <ul className="menu bg-base-200/50 rounded-box">
              {albums.map((album) => (
                <li key={album.album_id}>
                  <button
                    className="flex items-center gap-3"
                    onClick={() => void handleSelectAlbum(album.album_id!)}
                    disabled={isAddingToAlbum}
                  >
                    <div className="size-10 rounded-box overflow-hidden bg-base-300 flex-shrink-0 flex items-center justify-center opacity-40">
                      <Plus size={18} />
                    </div>
                    <div className="flex-1 min-w-0 text-left">
                      <div className="font-medium text-sm truncate">{album.album_name}</div>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <div className="text-center py-12 opacity-50">
              <p>
                {t("assets.assetsPageHeader.addToAlbumModal.noAlbumsFound", {
                  defaultValue: "No albums found",
                })}
              </p>
            </div>
          )}

          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost btn-sm">{t("common.cancel")}</button>
            </form>
          </div>
        </div>
      </dialog>

      <div className="fab fab-flower">
        <div
          tabIndex={0}
          role="button"
          className="btn btn-circle btn-lg"
          aria-label={t("assets.assetsPageHeader.moreActions")}
        >
          <Ellipsis />
        </div>
        <div className="fab-close">
          <span className="btn btn-circle btn-lg btn-error">✕</span>
        </div>
        <button
          type="button"
          className={`btn btn-circle btn-lg ${showInfo ? "btn-primary" : ""}`}
          onClick={onToggleInfo}
          aria-label={t("assets.mediaViewer.toggleInfo", "Toggle asset information")}
        >
          <Info />
        </button>
        <button
          type="button"
          className={`btn btn-circle btn-lg ${optimisticLiked ? "text-red-500" : ""}`}
          onClick={handleLikeToggle}
          disabled={!asset}
          aria-label={t("assets.mediaViewer.toggleLike", "Toggle liked status")}
        >
          <Heart className={optimisticLiked ? "fill-red-500" : ""} />
        </button>
        <button
          type="button"
          className="btn btn-circle btn-lg"
          onClick={() => openDialog("asset_export_dialog")}
          disabled={!asset}
          aria-label={t("exportModal.share")}
        >
          <Share />
        </button>
        <button
          type="button"
          className="btn btn-circle btn-lg text-error"
          onClick={() => openDialog(DELETE_DIALOG_ID)}
          disabled={!deleteTarget || isDeleting}
          aria-label={t("common.delete")}
        >
          <Trash2 />
        </button>
      </div>
    </>
  );
}
