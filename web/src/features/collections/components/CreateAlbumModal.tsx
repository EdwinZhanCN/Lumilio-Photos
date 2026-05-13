import React, { useState, useCallback } from "react";
import { useCollections } from "../CollectionsProvider";
import { useQueryClient } from "@tanstack/react-query";
import { useI18n } from "@/lib/i18n.tsx";
import { X, FolderPlus, Image as ImageIcon, MoveLeft } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult } from "@/lib/albums/types";
import { assetUrls } from "@/lib/assets/assetUrls";
import PhotoPicker from "@/components/PhotoPicker";

const CreateAlbumModal: React.FC = () => {
  const { t } = useI18n();
  const { isCreateModalOpen, dispatch } = useCollections();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedCoverId, setSelectedCoverId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isChoosingCover, setIsChoosingCover] = useState(false);
  const queryClient = useQueryClient();
  const createAlbumMutation = $api.useMutation("post", "/api/v1/albums");

  const handlePhotoSelect = useCallback((id: string) => {
    setSelectedCoverId(id);
    setIsChoosingCover(false);
  }, []);

  if (!isCreateModalOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !selectedCoverId) return;

    setIsSubmitting(true);
    try {
      const response = await createAlbumMutation.mutateAsync({
        body: {
          album_name: name,
          description: description,
          cover_asset_id: selectedCoverId,
        },
      });

      const responseData = response as ApiResult;
      if (responseData?.code === 0) {
        await queryClient.invalidateQueries({ queryKey: ["albums"] });
        dispatch({ type: "CLOSE_CREATE_MODAL" });
        resetForm();
      }
    } catch (error) {
      console.error("Failed to create album:", error);
    } finally {
      setIsSubmitting(false);
    }
  };

  const resetForm = () => {
    setName("");
    setDescription("");
    setSelectedCoverId(null);
    setIsChoosingCover(false);
  };

  const handleClose = () => {
    dispatch({ type: "CLOSE_CREATE_MODAL" });
    resetForm();
  };

  return (
    <div className="modal modal-open z-50">
      <div className="modal-box max-w-4xl h-[85vh] flex flex-col p-0 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 bg-base-200/50 border-b border-base-200 z-30">
          <div className="flex items-center gap-3 text-base-content">
            <FolderPlus size={24} className="text-primary" />
            <h3 className="font-bold text-lg">{t("collections.newAlbum")}</h3>
          </div>
          <button
            className="btn btn-sm btn-circle btn-ghost"
            onClick={handleClose}
          >
            <X size={20} />
          </button>
        </div>

        <div className="flex-1 overflow-hidden relative bg-base-100">
          <form onSubmit={handleSubmit} className="h-full flex flex-col">
            <div className="flex-1 overflow-y-auto p-6 md:p-8">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8 md:gap-12 h-full">
                {/* Left Column: Details */}
                <div className="flex flex-col gap-6 h-full">
                  <fieldset className="fieldset w-full">
                    <legend className="fieldset-legend font-semibold">
                      {t("collections.createModal.fields.name.label")}
                    </legend>
                    <input
                      type="text"
                      placeholder={t(
                        "collections.createModal.fields.name.placeholder",
                      )}
                      className="input input-bordered w-full"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      required
                    />
                  </fieldset>

                  <fieldset className="fieldset w-full flex-1 h-full min-h-0">
                    <legend className="fieldset-legend font-semibold">
                      {t("collections.createModal.fields.description.label")}
                    </legend>
                    <textarea
                      className="textarea textarea-bordered w-full h-full min-h-30 resize-none flex-1"
                      placeholder={t(
                        "collections.createModal.fields.description.placeholder",
                      )}
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                    ></textarea>
                  </fieldset>
                </div>

                {/* Right Column: Cover Selection Preview */}
                <fieldset className="fieldset w-full h-full flex flex-col min-h-0">
                  <legend className="fieldset-legend font-semibold">
                    {t("collections.createModal.fields.cover.label")}
                  </legend>

                  <div
                    className={`flex-1 w-full min-h-62.5 border-2 border-dashed rounded-box flex flex-col items-center justify-center gap-4 transition-all cursor-pointer overflow-hidden relative group
                      ${selectedCoverId ? "border-primary bg-base-100 shadow-sm" : "border-base-300 bg-base-200/30 hover:border-primary/50 hover:bg-base-200/60"}
                    `}
                    onClick={() => setIsChoosingCover(true)}
                  >
                    {selectedCoverId ? (
                      <>
                        <img
                          src={assetUrls.getThumbnailUrl(
                            selectedCoverId,
                            "medium",
                          )}
                          className="w-full h-full object-cover"
                          alt={t(
                            "collections.createModal.fields.cover.selectedAlt",
                          )}
                        />
                        <div className="absolute inset-0 bg-base-300/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center backdrop-blur-[2px]">
                          <span className="btn btn-primary btn-sm rounded-full shadow-lg">
                            {t("collections.createModal.fields.cover.change")}
                          </span>
                        </div>
                      </>
                    ) : (
                      <>
                        <div className="p-4 bg-primary/10 rounded-full text-primary transition-transform group-hover:scale-110 duration-300">
                          <ImageIcon size={32} />
                        </div>
                        <div className="text-center px-4">
                          <span className="block text-sm font-bold text-base-content">
                            {t("collections.createModal.fields.cover.choose")}
                          </span>
                          <span className="text-xs text-base-content/60 mt-1 block">
                            {t(
                              "collections.createModal.fields.cover.requiredHint",
                            )}
                          </span>
                        </div>
                      </>
                    )}
                  </div>
                </fieldset>
              </div>
            </div>

            {/* Footer */}
            <div className="p-4 md:p-6 bg-base-200/50 border-t border-base-200 flex justify-end gap-3 mt-auto">
              <button
                type="button"
                className="btn btn-ghost"
                onClick={handleClose}
              >
                {t("common.cancel")}
              </button>
              <button
                type="submit"
                className="btn btn-primary"
                disabled={isSubmitting || !name.trim() || !selectedCoverId}
              >
                {isSubmitting && (
                  <span className="loading loading-spinner loading-sm"></span>
                )}
                {isSubmitting
                  ? t("collections.createModal.actions.creating")
                  : t("collections.createModal.actions.create")}
              </button>
            </div>
          </form>

          {/* Full Assets View Photo Picker Overlay */}
          {isChoosingCover && (
            <div className="absolute inset-0 bg-base-100 z-40 flex flex-col animate-in slide-in-from-bottom duration-300">
              <div className="flex items-center p-3 bg-base-100 border-b border-base-200 sticky top-0 z-50 shadow-sm">
                <button
                  type="button"
                  className="btn btn-sm btn-ghost btn-circle"
                  onClick={() => setIsChoosingCover(false)}
                >
                  <MoveLeft size={20} />
                </button>
              </div>

              <div className="flex-1 overflow-hidden">
                <PhotoPicker
                  scopeId="photo-picker:album-cover"
                  onSelect={handlePhotoSelect}
                />
              </div>
            </div>
          )}
        </div>
      </div>
      <div
        className="modal-backdrop bg-base-300/60 backdrop-blur-sm"
        onClick={handleClose}
      ></div>
    </div>
  );
};

export default CreateAlbumModal;
