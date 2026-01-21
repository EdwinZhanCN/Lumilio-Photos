import React, { useState, useEffect, useCallback } from "react";
import { useCollections } from "../CollectionsProvider";
import { albumService } from "@/services/albumService";
import { useQueryClient } from "@tanstack/react-query";
import { useI18n } from "@/lib/i18n.tsx";
import {X, FolderPlus, Image as ImageIcon, MoveLeft} from "lucide-react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { useSelection } from "@/features/assets/hooks/useSelection";

const PhotoPicker: React.FC<{ onSelect: (id: string) => void }> = ({ onSelect }) => {
  const { state, dispatch } = useAssetsContext();
  const { groupBy } = state.ui;
  const selection = useSelection();

  const {
    assets: allAssets,
    groups: groupedAssets,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
  } = useCurrentTabAssets({
    withGroups: true,
    groupBy,
  });

  // Initialize picker state
  useEffect(() => {
    // Force clear everything on mount
    dispatch({ type: "CLEAR_SELECTION" });
    dispatch({ type: "RESET_FILTERS" });
    dispatch({ type: "SET_SEARCH_QUERY", payload: "" });
    
    selection.setEnabled(true);
    selection.setSelectionMode("single");
  }, []);

  // Sync selection with parent
  useEffect(() => {
    if (selection.enabled && selection.selectedCount > 0) {
      const id = Array.from(selection.selectedIds)[0];
      if (id) {
        onSelect(id);
      }
    }
  }, [selection.selectedIds, selection.enabled, onSelect]);

  return (
    <div className="flex flex-col h-full bg-base-100 overflow-hidden">
      <AssetsPageHeader
        groupBy={groupBy}
        onGroupByChange={(v) => dispatch({ type: "SET_GROUP_BY", payload: v })}
      />
      <div className="flex-1 overflow-y-auto overflow-x-hidden custom-scrollbar">
        <JustifiedGallery
          groupedPhotos={groupedAssets || {}}
          openCarousel={() => {}} 
          onLoadMore={fetchMore}
          hasMore={hasMore}
          isLoadingMore={isLoadingMore}
          isLoading={isLoading && allAssets.length === 0}
        />
      </div>
    </div>
  );
};

const CreateAlbumModal: React.FC = () => {
  const { t } = useI18n();
  const { isCreateModalOpen, dispatch } = useCollections();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedCoverId, setSelectedCoverId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isChoosingCover, setIsChoosingCover] = useState(false);
  const queryClient = useQueryClient();

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
      const response = await albumService.createAlbum({
        album_name: name,
        description: description,
        cover_asset_id: selectedCoverId,
      });

      if (response.data.code === 0) {
        queryClient.invalidateQueries({ queryKey: ["albums"] });
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
      <div className="modal-box max-w-4xl h-[85vh] flex flex-col p-0 overflow-hidden shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between p-6 bg-base-100 z-30">
          <div className="flex items-center gap-2 text-primary">
            <FolderPlus size={24} />
            <h3 className="font-bold text-lg">{t("collections.newAlbum")}</h3>
          </div>
          <button className="btn btn-sm btn-circle btn-ghost" onClick={handleClose}>
            <X size={20} />
          </button>
        </div>

        <div className="flex-1 overflow-hidden relative bg-base-200/30">
          <form onSubmit={handleSubmit} className="h-full flex flex-col p-8 space-y-8 overflow-y-auto">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-10">
              {/* Left Column: Details */}
              <div className="space-y-6">
                <div className="form-control w-full">
                  <label className="label">
                    <span className="label-text font-bold text-base-content/70 uppercase tracking-wider text-xs">Album Name</span>
                  </label>
                  <input
                    type="text"
                    placeholder="e.g. Summer Vacation 2024"
                    className="input input-bordered w-full focus:input-primary bg-base-100"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                  />
                </div>

                <div className="form-control w-full">
                  <label className="label">
                    <span className="label-text font-bold text-base-content/70 uppercase tracking-wider text-xs">Description</span>
                  </label>
                  <textarea
                    className="textarea textarea-bordered h-40 focus:textarea-primary resize-none bg-base-100"
                    placeholder="Tell a story about this collection..."
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                  ></textarea>
                </div>
              </div>

              {/* Right Column: Cover Selection Preview */}
              <div className="flex flex-col">
                <label className="label">
                  <span className="label-text font-bold text-base-content/70 uppercase tracking-wider text-xs">Album Cover</span>
                </label>
                
                <div 
                  className={`flex-1 min-h-[250px] border-2 border-dashed rounded-2xl flex flex-col items-center justify-center gap-4 transition-all cursor-pointer overflow-hidden relative group
                    ${selectedCoverId ? 'border-primary bg-base-100 shadow-inner' : 'border-base-300 bg-base-100 hover:border-primary/50 hover:bg-base-200/50'}
                  `}
                  onClick={() => setIsChoosingCover(true)}
                >
                  {selectedCoverId ? (
                    <>
                      <img 
                        src={`http://localhost:8080/api/v1/assets/${selectedCoverId}/thumbnail?size=medium`} 
                        className="w-full h-full object-cover"
                        alt="Selected cover"
                      />
                      <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center backdrop-blur-[2px]">
                        <span className="text-white text-sm font-bold bg-primary px-4 py-2 rounded-full shadow-lg">Change Cover</span>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className="p-5 bg-primary/5 rounded-full text-primary transition-colors group-hover:scale-110 duration-300">
                        <ImageIcon size={40} />
                      </div>
                      <div className="text-center">
                        <span className="block text-sm font-bold text-base-content/70">Choose a cover photo</span>
                        <span className="text-xs text-base-content/40 mt-1 block">Required for new albums</span>
                      </div>
                    </>
                  )}
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-4 pt-6 mt-auto">
              <button type="button" className="btn btn-ghost px-8" onClick={handleClose}>
                Cancel
              </button>
              <button 
                type="submit" 
                className={`btn btn-primary px-12 shadow-lg shadow-primary/20 ${isSubmitting ? 'loading' : ''}`}
                disabled={isSubmitting || !name.trim() || !selectedCoverId}
              >
                {isSubmitting ? 'Creating...' : 'Create Album'}
              </button>
            </div>
          </form>

          {/* Full Assets View Photo Picker Overlay */}
          {isChoosingCover && (
            <div className="absolute inset-0 bg-base-100 z-40 flex flex-col animate-in slide-in-from-bottom duration-300">
              <div className="flex items-center justify-between p-4 bg-base-100 sticky top-0 z-50 shadow-sm">
                <button 
                  type="button"
                  className="btn btn-sm btn-soft btn-accent w-full"
                  onClick={() => setIsChoosingCover(false)}
                >
                  <MoveLeft size={20} />
                </button>
              </div>
              
              <div className="flex-1 overflow-hidden">
                <AssetsProvider persist={false}>
                  <PhotoPicker onSelect={handlePhotoSelect} />
                </AssetsProvider>
              </div>
            </div>
          )}
        </div>
      </div>
      <div className="modal-backdrop bg-black/60 backdrop-blur-sm" onClick={handleClose}></div>
    </div>
  );
};

export default CreateAlbumModal;
