import React, { useCallback, useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Bird, FolderPen, FolderPlus, Image as ImageIcon, MoveLeft } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import PhotoPicker from "@/components/PhotoPicker";
import Modal from "@/components/Modal";
import type { Album } from "@/lib/albums/types";

interface AlbumFormModalProps {
  open: boolean;
  mode: "create" | "edit";
  /** The album being edited; required when `mode === "edit"`. */
  album?: Album | null;
  onClose: () => void;
  /** Called after a successful create/edit, after caches are invalidated. */
  onSaved?: () => void;
}

/**
 * Create or edit an album. One controlled modal serves both flows so the
 * create and edit experiences stay identical. Drives `POST /albums` in create
 * mode and `PATCH /albums/{id}` in edit mode; reuses {@link PhotoPicker} for
 * the cover via an overlay over the modal body.
 */
export function AlbumFormModal({
  open,
  mode,
  album,
  onClose,
  onSaved,
}: AlbumFormModalProps): React.ReactNode {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const showMessage = useMessage();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [coverId, setCoverId] = useState<string | null>(null);
  const [isBioAlbum, setIsBioAlbum] = useState(false);
  const [isChoosingCover, setIsChoosingCover] = useState(false);

  const createMutation = $api.useMutation("post", "/api/v1/albums");
  const updateMutation = $api.useMutation("put", "/api/v1/albums/{id}");
  const isSubmitting = createMutation.isPending || updateMutation.isPending;

  // Seed the form whenever the modal opens (or targets a different album).
  const seedKey = open ? `${mode}:${album?.album_id ?? "new"}` : "closed";
  useEffect(() => {
    if (!open) return;
    if (mode === "edit" && album) {
      setName(album.album_name ?? "");
      setDescription(album.description ?? "");
      setCoverId(album.cover_asset_id ?? album.display_cover_asset_id ?? null);
      setIsBioAlbum(album.album_type === "bio");
    } else {
      setName("");
      setDescription("");
      setCoverId(null);
      setIsBioAlbum(false);
    }
    setIsChoosingCover(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedKey]);

  const handlePhotoSelect = useCallback((id: string) => {
    setCoverId(id);
    setIsChoosingCover(false);
  }, []);

  const close = () => {
    if (isSubmitting) return;
    onClose();
  };

  const canSubmit =
    name.trim().length > 0 && (mode === "edit" || Boolean(coverId)) && !isSubmitting;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    try {
      if (mode === "edit" && album?.album_id != null) {
        await updateMutation.mutateAsync({
          params: { path: { id: album.album_id } },
          body: {
            album_name: name.trim(),
            description,
            album_type: isBioAlbum ? "bio" : "default",
            ...(coverId ? { cover_asset_id: coverId } : {}),
          },
        });
      } else {
        await createMutation.mutateAsync({
          body: {
            album_name: name.trim(),
            description,
            cover_asset_id: coverId ?? undefined,
            album_type: isBioAlbum ? "bio" : undefined,
          },
        });
      }
      await queryClient.invalidateQueries({ queryKey: ["albums"] });
      if (mode === "edit" && album?.album_id != null) {
        await queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/albums/{id}"],
        });
      }
      onSaved?.();
      onClose();
    } catch (error) {
      console.error("Failed to save album:", error);
      showMessage(
        "error",
        mode === "edit"
          ? t("collections.albumForm.saveError", "Failed to save album.")
          : t("collections.messages.createError", "Failed to create album."),
      );
    }
  };

  const isEdit = mode === "edit";

  const footer = (
    <>
      <button type="button" className="btn btn-ghost" onClick={close} disabled={isSubmitting}>
        {t("common.cancel")}
      </button>
      <button
        type="submit"
        form="album-form"
        className="btn btn-primary"
        disabled={!canSubmit}
      >
        {isSubmitting && <span className="loading loading-spinner loading-sm" />}
        {isSubmitting
          ? isEdit
            ? t("collections.albumForm.saving", "Saving...")
            : t("collections.createModal.actions.creating")
          : isEdit
            ? t("collections.albumForm.save", "Save changes")
            : t("collections.createModal.actions.create")}
      </button>
    </>
  );

  return (
    <Modal
      open={open}
      onClose={close}
      size="lg"
      className="h-[85vh]"
      dismissable={!isChoosingCover && !isSubmitting}
      icon={isEdit ? <FolderPen size={22} /> : <FolderPlus size={22} />}
      title={
        isEdit
          ? t("collections.albumForm.editTitle", "Edit album")
          : t("collections.newAlbum")
      }
      footer={footer}
    >
      <form id="album-form" onSubmit={handleSubmit} className="h-full">
        <div className="grid h-full grid-cols-1 gap-8 p-6 md:grid-cols-2 md:gap-12 md:p-8">
          {/* Left: text fields */}
          <div className="flex h-full flex-col gap-6">
            <fieldset className="fieldset w-full">
              <legend className="fieldset-legend font-semibold">
                {t("collections.createModal.fields.name.label")}
              </legend>
              <input
                type="text"
                placeholder={t("collections.createModal.fields.name.placeholder")}
                className="input input-bordered w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </fieldset>

            <fieldset className="fieldset h-full min-h-0 w-full flex-1">
              <legend className="fieldset-legend font-semibold">
                {t("collections.createModal.fields.description.label")}
              </legend>
              <textarea
                className="textarea textarea-bordered h-full min-h-30 w-full flex-1 resize-none"
                placeholder={t("collections.createModal.fields.description.placeholder")}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </fieldset>

            <div className="space-y-2">
              <button
                type="button"
                className={`btn w-fit gap-2 ${isBioAlbum ? "btn-primary" : "btn-outline"}`}
                aria-pressed={isBioAlbum}
                onClick={() => setIsBioAlbum((v) => !v)}
              >
                <Bird className="size-4" />
                {t("collections.createModal.fields.bioAlbum.label")}
              </button>
              <p className="text-xs leading-relaxed text-base-content/60">
                {t("collections.createModal.fields.bioAlbum.hint")}
              </p>
            </div>
          </div>

          {/* Right: cover */}
          <fieldset className="fieldset flex h-full min-h-0 w-full flex-col">
            <legend className="fieldset-legend font-semibold">
              {t("collections.createModal.fields.cover.label")}
            </legend>
            <button
              type="button"
              onClick={() => setIsChoosingCover(true)}
              className={`group relative flex min-h-62.5 w-full flex-1 flex-col items-center justify-center gap-4 overflow-hidden rounded-box border-2 border-dashed transition-all ${
                coverId
                  ? "border-primary bg-base-100 shadow-sm"
                  : "border-base-300 bg-base-200/30 hover:border-primary/50 hover:bg-base-200/60"
              }`}
            >
              {coverId ? (
                <>
                  <img
                    src={assetUrls.getThumbnailUrl(coverId, "medium")}
                    className="h-full w-full object-cover"
                    alt={t("collections.createModal.fields.cover.selectedAlt")}
                  />
                  <div className="absolute inset-0 flex items-center justify-center bg-base-300/40 opacity-0 backdrop-blur-[2px] transition-opacity group-hover:opacity-100">
                    <span className="btn btn-primary btn-sm rounded-full shadow-lg">
                      {t("collections.createModal.fields.cover.change")}
                    </span>
                  </div>
                </>
              ) : (
                <>
                  <div className="rounded-full bg-primary/10 p-4 text-primary transition-transform duration-300 group-hover:scale-110">
                    <ImageIcon size={32} />
                  </div>
                  <div className="px-4 text-center">
                    <span className="block text-sm font-bold text-base-content">
                      {t("collections.createModal.fields.cover.choose")}
                    </span>
                    <span className="mt-1 block text-xs text-base-content/60">
                      {t("collections.createModal.fields.cover.requiredHint")}
                    </span>
                  </div>
                </>
              )}
            </button>
          </fieldset>
        </div>
      </form>

      {isChoosingCover && (
        <div className="absolute inset-0 z-40 flex flex-col bg-base-100">
          <div className="sticky top-0 z-50 flex items-center border-b border-base-200 bg-base-100 p-3 shadow-sm">
            <button
              type="button"
              className="btn btn-circle btn-ghost btn-sm"
              onClick={() => setIsChoosingCover(false)}
            >
              <MoveLeft size={20} />
            </button>
          </div>
          <div className="flex-1 overflow-hidden">
            <PhotoPicker scopeId="photo-picker:album-cover" onSelect={handlePhotoSelect} />
          </div>
        </div>
      )}
    </Modal>
  );
}

export default AlbumFormModal;
