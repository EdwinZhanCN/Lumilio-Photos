import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { ErrorBoundary } from "react-error-boundary";
import {
  Album,
  Plus,
  Trash2,
  X,
  SquareMousePointer,
  AlertTriangle,
} from "lucide-react";
import { ImgStackGrid } from "../components/ImgStackGrid";
import { useI18n } from "@/lib/i18n.tsx";
import { useAlbums } from "../hooks/useAlbums";
import { useNavigate } from "react-router-dom";
import { CollectionsProvider, useCollections } from "../CollectionsProvider";
import CreateAlbumModal from "../components/CreateAlbumModal";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { $api } from "@/lib/http-commons/queryClient";
import { useWorkingRepository } from "@/features/settings";

function CollectionsContent() {
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

  // Flatten all pages into a single array
  const albums = data?.pages.flatMap((page) => page.albums) ?? [];

  const handleAlbumClick = (album: { id: string }) => {
    if (isSelectionMode) {
      const id = parseInt(album.id);
      if (selectedAlbumIds.includes(id)) {
        dispatch({ type: "DESELECT_ALBUM", payload: id });
      } else {
        dispatch({ type: "SELECT_ALBUM", payload: id });
      }
      return;
    }
    navigate(`/collections/${album.id}`);
  };

  const toggleSelectionMode = () => {
    dispatch({ type: "TOGGLE_SELECTION_MODE" });
  };

  const handleCreateAlbum = () => {
    dispatch({ type: "OPEN_CREATE_MODAL" });
  };

  const handleDeleteSelected = () => {
    setIsDeleteConfirmOpen(true);
  };

  const confirmDelete = async () => {
    setIsDeleting(true);
    try {
      // Bulk delete albums
      await Promise.all(
        selectedAlbumIds.map((id) =>
          deleteAlbumMutation.mutateAsync({ params: { path: { id } } }),
        ),
      );

      await queryClient.invalidateQueries({ queryKey: ["albums"] });
      showMessage(
        "success",
        t("collections.messages.deleteSuccess"),
      );
      dispatch({ type: "CLEAR_SELECTION" });
      dispatch({ type: "TOGGLE_SELECTION_MODE" });
      setIsDeleteConfirmOpen(false);
    } catch (error) {
      console.error("Failed to delete albums:", error);
      showMessage(
        "error",
        t("collections.messages.deleteError"),
      );
    } finally {
      setIsDeleting(false);
    }
  };

  // Show loading skeleton only on initial load when there's no data
  const showLoading = isPending && albums.length === 0;

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t("routes.collections")}
        icon={<Album className="w-6 h-6 text-primary" strokeWidth={1.5} />}
      >
        <div className="flex items-center gap-2 min-h-[32px]">
          {isSelectionMode ? (
            <>
              <span className="text-sm font-medium mr-2">
                {t("common.selected", { count: selectedAlbumIds.length })}
              </span>
              <button
                className={`btn btn-sm btn-circle btn-error ${selectedAlbumIds.length === 0 ? "btn-disabled opacity-50" : ""}`}
                onClick={handleDeleteSelected}
                disabled={selectedAlbumIds.length === 0}
                title={t("common.delete")}
              >
                <Trash2 className="w-4 h-4" />
              </button>
              <button
                className="btn btn-soft btn-sm btn-circle btn-accent"
                onClick={toggleSelectionMode}
                title={t("common.cancel")}
              >
                <X className="w-4 h-4" />
              </button>
            </>
          ) : (
            <>
              <button
                className="btn btn-sm btn-soft btn-info"
                onClick={handleCreateAlbum}
              >
                <Plus className="w-4 h-4" />
                {t("collections.newAlbum")}
              </button>
              <button
                className="btn btn-sm btn-soft btn-info btn-circle"
                onClick={toggleSelectionMode}
                title={t("common.select")}
              >
                <SquareMousePointer className="w-4 h-4" />
              </button>
            </>
          )}
        </div>
      </PageHeader>

      <div className="flex-1 overflow-y-auto p-4 min-h-0">
        {/* Show a subtle loading indicator when refetching or invalidating */}
        {isFetching && !isPending && !isFetchingNextPage && (
          <div className="absolute top-20 right-8 z-20">
            <span className="loading loading-spinner loading-sm text-primary opacity-50"></span>
          </div>
        )}

        <ImgStackGrid
          albums={albums}
          onAlbumClick={handleAlbumClick}
          loading={showLoading}
          selectedIds={selectedAlbumIds.map(String)}
          isSelectionMode={isSelectionMode}
        />

        {hasNextPage && (
          <div className="flex justify-center p-8 min-h-[100px]">
            <button
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
              className="btn btn-outline btn-wide"
            >
              {isFetchingNextPage
                ? t("common.loading")
                : t("common.loadMore")}
            </button>
          </div>
        )}
      </div>

      <CreateAlbumModal />

      {/* Delete Confirmation Modal */}
      {isDeleteConfirmOpen && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-error">
            <div className="flex items-center gap-3 text-error mb-4">
              <AlertTriangle size={24} />
              <h3 className="font-bold text-lg">
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
          ></div>
        </div>
      )}
    </div>
  );
}

function Collections() {
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
        <CollectionsContent />
      </CollectionsProvider>
    </ErrorBoundary>
  );
}

export default Collections;
