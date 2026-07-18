import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n";
import { AlertTriangle, FolderPlus, Heart, Plus, Star, Tags, Trash2, X } from "lucide-react";
import TagPickerMenu from "../../../components/TagPickerMenu";
import type { AssetsPageHeaderBulkActions } from "./useAssetsPageHeaderBulkActions";

interface AssetsPageHeaderModalsProps {
  bulk: AssetsPageHeaderBulkActions;
}

export default function AssetsPageHeaderModals({ bulk }: AssetsPageHeaderModalsProps) {
  const { t } = useI18n();
  const {
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
    selectedItemCount,
    showAffectedAssetCount,
    renderAffectedAssetHint,
  } = bulk;

  return (
    <>
      {confirmableBulkAction && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-primary">
            <div className="mb-4 flex items-center gap-3 text-primary">
              {confirmableBulkAction.type === "rating" ? <Star size={24} /> : <Heart size={24} />}
              <h3 className="text-lg font-bold">
                {confirmableBulkAction.type === "rating"
                  ? t("assets.assetsPageHeader.bulkConfirm.ratingTitle", {
                      rating: confirmableBulkAction.rating,
                      defaultValue:
                        confirmableBulkAction.rating === 0
                          ? "Set selected assets as unrated?"
                          : "Set selected assets to {{rating}} stars?",
                    })
                  : t("assets.assetsPageHeader.bulkConfirm.likedTitle", {
                      action: confirmableBulkAction.liked
                        ? t("assets.assetsPageHeader.actions.like", {
                            defaultValue: "Liked",
                          })
                        : t("assets.assetsPageHeader.actions.unlike", {
                            defaultValue: "Unliked",
                          }),
                      defaultValue: "Update liked status?",
                    })}
              </h3>
            </div>
            <p className="py-4 text-sm text-base-content/70">{renderAffectedAssetHint()}</p>
            <div className="modal-action">
              <button className="btn btn-ghost" onClick={() => setConfirmableBulkAction(null)}>
                {t("common.cancel")}
              </button>
              <button className="btn btn-primary" onClick={confirmBulkAction}>
                {t("common.confirm", { defaultValue: "Confirm" })}
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={() => setConfirmableBulkAction(null)}></div>
        </div>
      )}

      {confirmableCustomAction && (
        <div className="modal modal-open">
          <div
            className={`modal-box border-t-4 ${
              confirmableCustomAction.tone === "danger" ? "border-error" : "border-primary"
            }`}
          >
            <div
              className={`mb-4 flex items-center gap-3 ${
                confirmableCustomAction.tone === "danger" ? "text-error" : "text-primary"
              }`}
            >
              {confirmableCustomAction.icon ?? <AlertTriangle size={24} />}
              <h3 className="text-lg font-bold">
                {confirmableCustomAction.confirmationTitle ?? confirmableCustomAction.label}
              </h3>
            </div>
            <p className="py-4 text-sm text-base-content/70">
              {confirmableCustomAction.confirmationMessage ?? renderAffectedAssetHint()}
            </p>
            <div className="modal-action">
              <button
                className="btn btn-ghost"
                disabled={isRunningCustomAction}
                onClick={() => setConfirmableCustomAction(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className={`btn gap-2 ${
                  confirmableCustomAction.tone === "danger" ? "btn-error" : "btn-primary"
                }`}
                disabled={isRunningCustomAction}
                onClick={() => void executeCustomBulkAction(confirmableCustomAction)}
              >
                {isRunningCustomAction ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  confirmableCustomAction.icon
                )}
                {t("common.confirm", { defaultValue: "Confirm" })}
              </button>
            </div>
          </div>
          <div
            className="modal-backdrop"
            onClick={() => {
              if (!isRunningCustomAction) {
                setConfirmableCustomAction(null);
              }
            }}
          ></div>
        </div>
      )}

      {isDeleteConfirmOpen && (
        <div className="modal modal-open">
          <div className="modal-box border-t-4 border-error">
            <div className="flex items-center gap-3 text-error mb-4">
              <AlertTriangle size={24} />
              <h3 className="font-bold text-lg">
                {t("assets.assetsPageHeader.deleteConfirmModal.title")}
              </h3>
            </div>
            <p className="py-4">
              {t("assets.assetsPageHeader.deleteConfirmModal.message", {
                count: selectedItemCount,
              })}
              {showAffectedAssetCount && (
                <span className="mt-2 block text-sm text-base-content/60">
                  {renderAffectedAssetHint()}
                </span>
              )}
            </p>
            <div className="modal-action">
              <button className="btn btn-ghost" onClick={() => setIsDeleteConfirmOpen(false)}>
                {t("assets.assetsPageHeader.deleteConfirmModal.cancelButton")}
              </button>
              <button className="btn btn-error gap-2" onClick={confirmDelete}>
                <Trash2 size={18} />
                {t("assets.assetsPageHeader.deleteConfirmModal.deleteButton")}
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={() => setIsDeleteConfirmOpen(false)}></div>
        </div>
      )}

      {isAlbumModalOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-md h-[80vh] flex flex-col p-0 overflow-hidden">
            <div className="flex items-center justify-between px-5 py-3 border-b border-base-200 shrink-0">
              <h3 className="font-bold text-lg flex items-center gap-2">
                <FolderPlus className="text-primary" size={20} />
                {t("assets.assetsPageHeader.addToAlbumModal.title")}
              </h3>
              <button
                className="btn btn-sm btn-circle btn-ghost"
                onClick={() => setIsAlbumModalOpen(false)}
              >
                <X size={20} />
              </button>
            </div>

            <p className="text-sm opacity-70 px-5 py-2 shrink-0">
              {t("assets.assetsPageHeader.addToAlbumModal.message", {
                count: selectedItemCount,
              })}
              {showAffectedAssetCount && (
                <span className="mt-1 block">{renderAffectedAssetHint()}</span>
              )}
            </p>

            <div className="flex-1 min-h-0 overflow-y-auto custom-scrollbar px-3">
              {isLoadingAlbums ? (
                <div className="flex justify-center py-12">
                  <span className="loading loading-spinner loading-md text-primary"></span>
                </div>
              ) : albums.length > 0 ? (
                <ul className="list bg-base-200/50 rounded-box">
                  <li className="p-4 pb-2 text-xs opacity-60 tracking-wide">
                    {t("assets.assetsPageHeader.addToAlbumModal.itemCount", {
                      count: albums.length,
                    })}
                  </li>
                  {albums.map((album) => (
                    <li key={album.album_id} className="list-row">
                      <div className="size-10 rounded-box overflow-hidden bg-base-300 flex-shrink-0">
                        {album.cover_asset_id ? (
                          <img
                            src={assetUrls.getThumbnailUrl(album.cover_asset_id, "small")}
                            className="size-full object-cover"
                            alt=""
                          />
                        ) : (
                          <div className="size-full flex items-center justify-center opacity-30">
                            <FolderPlus size={18} />
                          </div>
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="font-semibold text-sm truncate">{album.album_name}</div>
                        <div className="text-xs opacity-50">
                          {t("assets.assetsPageHeader.addToAlbumModal.itemCount", {
                            count: album.asset_count || 0,
                          })}
                        </div>
                      </div>
                      <button
                        className="btn btn-sm btn-ghost btn-square"
                        onClick={() => handleSelectAlbum(album.album_id!)}
                        disabled={isAddingToAlbum}
                      >
                        <Plus size={18} />
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <div className="text-center py-12 opacity-50">
                  <p>{t("assets.assetsPageHeader.addToAlbumModal.noAlbumsFound")}</p>
                </div>
              )}
            </div>

            <div className="border-t border-base-200 px-5 py-3 shrink-0">
              <button className="btn btn-ghost btn-sm" onClick={() => setIsAlbumModalOpen(false)}>
                {t("common.cancel")}
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={() => setIsAlbumModalOpen(false)}></div>
        </div>
      )}

      {isTagsModalOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-md flex flex-col p-0 overflow-hidden">
            <div className="flex items-center justify-between px-5 py-3 border-b border-base-200 shrink-0">
              <h3 className="font-bold text-lg flex items-center gap-2">
                <Tags className="text-primary" size={20} />
                {t("assets.assetsPageHeader.addTagsModal.title", {
                  defaultValue: "Add tags",
                })}
              </h3>
              <button className="btn btn-sm btn-circle btn-ghost" onClick={closeTagsModal}>
                <X size={20} />
              </button>
            </div>

            <p className="text-sm opacity-70 px-5 py-2 shrink-0">
              {t("assets.assetsPageHeader.addTagsModal.message", {
                count: selectedItemCount,
                defaultValue: "Add tags to {{count}} selected items.",
              })}
              {showAffectedAssetCount && (
                <span className="mt-1 block">{renderAffectedAssetHint()}</span>
              )}
            </p>

            <div className="px-3 pb-3">
              <TagPickerMenu
                query={tagQuery}
                onQueryChange={setTagQuery}
                checked={pendingTags}
                suggestions={tagSuggestionItems}
                onToggleChecked={removePendingTag}
                onSelectSuggestion={addPendingTag}
                showCreate={showCreateTag}
                createLabel={t("assets.assetsPageHeader.addTagsModal.create", {
                  name: trimmedTagQuery,
                  defaultValue: 'Create "{{name}}"',
                })}
                createName={trimmedTagQuery}
                onCreate={handleCreatePendingTag}
                loading={tagSuggestionsQuery.isFetching}
                placeholder={t("assets.assetsPageHeader.addTagsModal.searchPlaceholder", {
                  defaultValue: "Search or create tags…",
                })}
                loadingText={t("assets.assetsPageHeader.addTagsModal.loading", {
                  defaultValue: "Loading tags…",
                })}
                noResultsText={t("assets.assetsPageHeader.addTagsModal.noResults", {
                  defaultValue: "No matching tags",
                })}
                autoFocus
                onKeyDown={(event) => {
                  if (event.key === "Escape") {
                    event.preventDefault();
                    closeTagsModal();
                    return;
                  }
                  if (event.key === "Enter") {
                    event.preventDefault();
                    if (showCreateTag) {
                      handleCreatePendingTag();
                    } else if (tagSuggestionItems[0]) {
                      addPendingTag(tagSuggestionItems[0]);
                    }
                  }
                }}
                className="max-h-72"
              />
            </div>

            <div className="border-t border-base-200 px-5 py-3 shrink-0 flex justify-end gap-2">
              <button className="btn btn-ghost btn-sm" onClick={closeTagsModal}>
                {t("common.cancel")}
              </button>
              <button
                className="btn btn-primary btn-sm"
                disabled={pendingTags.length === 0 || isAddingTags}
                onClick={() => void handleApplyTags()}
              >
                {isAddingTags ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  t("assets.assetsPageHeader.addTagsModal.apply", {
                    count: pendingTags.length,
                    defaultValue: "Apply {{count}} tags",
                  })
                )}
              </button>
            </div>
          </div>
          <div className="modal-backdrop" onClick={closeTagsModal}></div>
        </div>
      )}
    </>
  );
}
