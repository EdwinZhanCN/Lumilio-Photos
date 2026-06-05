import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import {
  Album,
  Plus,
  Trash2,
  X,
  SquareMousePointer,
  AlertTriangle,
} from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { $api } from "@/lib/http-commons/queryClient";
import { useWorkingRepository } from "@/features/settings";
import { CollectionsProvider, useCollections } from "../CollectionsProvider";
import CreateAlbumModal from "../components/CreateAlbumModal";
import { ImgStackGrid } from "../components/ImgStackGrid";
import { useAlbums } from "../hooks/useAlbums";

function AlbumsContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { scopedRepositoryId } = useWorkingRepository();
  const { selectedAlbumIds, isSelectionMode, dispatch } = useCollections();
  const deleteAlbumMutation = $api.useMutation("delete", "/api/v1/albums/{id}");
  const {
    data,
    isPending,
    isFetching,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useAlbums(t, scopedRepositoryId);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const albums = data?.pages.flatMap((page) => page.albums) ?? [];

  const handleAlbumClick = (album: { id: string }) => {
    if (isSelectionMode) {
      const id = Number.parseInt(album.id, 10);
      if (selectedAlbumIds.includes(id)) {
        dispatch({ type: "DESELECT_ALBUM", payload: id });
      } else {
        dispatch({ type: "SELECT_ALBUM", payload: id });
      }
      return;
    }

    void navigate(`/collections/${album.id}`);
  };

  const confirmDelete = async () => {
    setIsDeleting(true);
    try {
      await Promise.all(
        selectedAlbumIds.map((id) =>
          deleteAlbumMutation.mutateAsync({ params: { path: { id } } }),
        ),
      );

      await queryClient.invalidateQueries({ queryKey: ["albums"] });
      showMessage("success", t("collections.messages.deleteSuccess"));
      dispatch({ type: "CLEAR_SELECTION" });
      dispatch({ type: "TOGGLE_SELECTION_MODE" });
      setIsDeleteConfirmOpen(false);
    } catch (error) {
      console.error("Failed to delete albums:", error);
      showMessage("error", t("collections.messages.deleteError"));
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.albums")}
        icon={<Album className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <div className="flex min-h-[32px] items-center gap-2">
          {isSelectionMode ? (
            <>
              <span className="mr-2 text-sm font-medium">
                {t("common.selected", { count: selectedAlbumIds.length })}
              </span>
              <button
                className={`btn btn-circle btn-sm btn-error ${selectedAlbumIds.length === 0 ? "btn-disabled opacity-50" : ""}`}
                onClick={() => setIsDeleteConfirmOpen(true)}
                disabled={selectedAlbumIds.length === 0}
                title={t("common.delete")}
              >
                <Trash2 className="h-4 w-4" />
              </button>
              <button
                className="btn btn-soft btn-sm btn-circle btn-accent"
                onClick={() => dispatch({ type: "TOGGLE_SELECTION_MODE" })}
                title={t("common.cancel")}
              >
                <X className="h-4 w-4" />
              </button>
            </>
          ) : (
            <>
              <button
                className="btn btn-sm btn-soft btn-info"
                onClick={() => dispatch({ type: "OPEN_CREATE_MODAL" })}
              >
                <Plus className="h-4 w-4" />
                {t("collections.newAlbum")}
              </button>
              <button
                className="btn btn-sm btn-soft btn-info btn-circle"
                onClick={() => dispatch({ type: "TOGGLE_SELECTION_MODE" })}
                title={t("common.select")}
              >
                <SquareMousePointer className="h-4 w-4" />
              </button>
            </>
          )}
        </div>
      </PageHeader>

      <div className="relative flex-1 min-h-0 overflow-y-auto p-4">
        {isFetching && !isPending && !isFetchingNextPage && (
          <div className="absolute right-8 top-20 z-20">
            <span className="loading loading-spinner loading-sm text-primary opacity-50" />
          </div>
        )}

        <ImgStackGrid
          albums={albums}
          onAlbumClick={handleAlbumClick}
          loading={isPending && albums.length === 0}
          selectedIds={selectedAlbumIds.map(String)}
          isSelectionMode={isSelectionMode}
        />

        {hasNextPage && (
          <div className="flex min-h-[100px] justify-center p-8">
            <button
              type="button"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
              className="btn btn-outline btn-wide"
            >
              {isFetchingNextPage ? t("common.loading") : t("common.loadMore")}
            </button>
          </div>
        )}
      </div>

      <CreateAlbumModal />

      {isDeleteConfirmOpen && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-error">
            <div className="mb-4 flex items-center gap-3 text-error">
              <AlertTriangle size={24} />
              <h3 className="text-lg font-bold">
                {t("collections.deleteModal.title")}
              </h3>
            </div>
            <p className="py-4">
              {t("collections.deleteModal.description", {
                count: selectedAlbumIds.length,
              })}
            </p>
            <div className="modal-action">
              <button
                className="btn btn-ghost"
                onClick={() => setIsDeleteConfirmOpen(false)}
              >
                {t("common.cancel")}
              </button>
              <button
                className={`btn btn-error gap-2 ${isDeleting ? "loading" : ""}`}
                onClick={confirmDelete}
                disabled={isDeleting}
              >
                {!isDeleting && <Trash2 size={18} />}
                {isDeleting
                  ? t("collections.deleteModal.deleting")
                  : t("collections.deleteModal.confirm")}
              </button>
            </div>
          </div>
          <div
            className="modal-backdrop"
            onClick={() => setIsDeleteConfirmOpen(false)}
          />
        </div>
      )}
    </div>
  );
}

export default function Albums() {
  const { t } = useI18n();

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <CollectionsProvider>
        <AlbumsContent />
      </CollectionsProvider>
    </ErrorBoundary>
  );
}
