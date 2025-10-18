import React, { ReactNode, useReducer, useCallback, useMemo } from "react";
import { useUploadProcess } from "@/hooks/api-hooks/useUploadProcess";
import { uploadReducer, initialState } from "./reducers";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { UploadContext } from "./types";
import { useSettingsContext } from "@/features/settings";
import { useGenerateThumbnail } from "@/hooks/util-hooks/useGenerateThumbnail";

export function UploadProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(uploadReducer, initialState);
  const showMessage = useMessage();
  const uploadProcess = useUploadProcess();
  const { state: settings } = useSettingsContext();
  const { isGenerating, progress, generatePreviews } = useGenerateThumbnail();

  // Get settings with defaults
  const maxPreviewCount = settings.ui.upload?.max_preview_count ?? 30;
  const maxTotalFiles = settings.ui.upload?.max_total_files ?? 100;

  // Calculate how many files currently have previews
  const previewCount = useMemo(
    () => state.previews.filter((p) => p !== "").length,
    [state.previews],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: true });
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: false });
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent, handleFiles?: (files: FileList) => void) => {
      e.preventDefault();
      dispatch({ type: "SET_DRAGGING", payload: false });
      const droppedFiles = e.dataTransfer?.files;
      if (handleFiles && droppedFiles?.length) {
        handleFiles(droppedFiles);
      }
    },
    [],
  );

  /**
   * Add files to the upload queue
   * @param files - Files to add
   * @param generatePreviewsFlag - Whether to generate preview thumbnails for these files
   */
  const addFiles = useCallback(
    async (files: File[], generatePreviewsFlag: boolean) => {
      if (files.length === 0) return;

      // Check total files limit
      const availableSlots = maxTotalFiles - state.files.length;
      if (availableSlots <= 0) {
        showMessage("error", `Maximum ${maxTotalFiles} files allowed`);
        return;
      }

      const filesToAdd = files.slice(0, availableSlots);
      if (files.length > availableSlots) {
        showMessage(
          "hint",
          `${files.length - availableSlots} files exceeded the limit and were removed`,
        );
      }

      // Determine how many previews to generate
      let previewsToGenerate = 0;
      if (generatePreviewsFlag) {
        const currentPreviewCount = state.previews.filter(
          (p) => p !== "",
        ).length;
        const availablePreviewSlots = maxPreviewCount - currentPreviewCount;
        previewsToGenerate = Math.min(filesToAdd.length, availablePreviewSlots);

        if (previewsToGenerate < filesToAdd.length) {
          showMessage(
            "info",
            `Generating previews for first ${previewsToGenerate} files (limit: ${maxPreviewCount})`,
          );
        }
      }

      const startIndex = state.files.length;

      // Add files with empty preview strings initially
      dispatch({
        type: "ADD_FILES",
        payload: {
          files: filesToAdd,
          previews: filesToAdd.map(() => ""),
        },
      });

      // Generate previews for the first N files if requested
      if (previewsToGenerate > 0) {
        try {
          const filesToPreview = filesToAdd.slice(0, previewsToGenerate);
          const result = await generatePreviews(filesToPreview);

          if (result instanceof Error) {
            throw result;
          }

          if (!result) {
            throw new Error("Preview generation returned undefined");
          }

          const previewUrls = result.map((p) => p?.url || "");

          dispatch({
            type: "UPDATE_PREVIEW_URLS",
            payload: {
              startIndex,
              urls: previewUrls,
            },
          });
        } catch (error) {
          console.error("Preview generation failed:", error);
          showMessage(
            "error",
            error instanceof Error
              ? error.message
              : "Failed to generate previews",
          );
        }
      }
    },
    [
      state.files.length,
      state.previews,
      maxTotalFiles,
      maxPreviewCount,
      showMessage,
      generatePreviews,
    ],
  );

  const clearFiles = useCallback(() => {
    dispatch({ type: "CLEAR_FILES" });
  }, []);

  const uploadFiles = useCallback(async () => {
    if (!state.files.length) {
      showMessage("info", "No files selected for upload.");
      return;
    }

    try {
      await uploadProcess.processFiles(state.files);
      dispatch({ type: "CLEAR_FILES" });
      showMessage("success", "Upload completed successfully!");
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Upload failed";
      showMessage("error", errorMessage);
    }
  }, [state.files, uploadProcess, showMessage]);

  const contextValue = useMemo(
    () => ({
      state,
      dispatch,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      addFiles,
      clearFiles,
      uploadFiles,
      isProcessing:
        uploadProcess.isGeneratingHashCodes || uploadProcess.isUploading,
      uploadProgress: uploadProcess.uploadProgress,
      hashcodeProgress: uploadProcess.hashcodeProgress,
      isGeneratingHashCodes: uploadProcess.isGeneratingHashCodes,
      isGeneratingPreviews: isGenerating,
      previewProgress: progress,
      fileProgress: uploadProcess.fileProgress,
      maxPreviewCount,
      maxTotalFiles,
      previewCount,
    }),
    [
      state,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      addFiles,
      clearFiles,
      uploadFiles,
      uploadProcess.isGeneratingHashCodes,
      uploadProcess.isUploading,
      uploadProcess.uploadProgress,
      uploadProcess.hashcodeProgress,
      uploadProcess.fileProgress,
      isGenerating,
      progress,
      maxPreviewCount,
      maxTotalFiles,
      previewCount,
    ],
  );

  return (
    <UploadContext.Provider value={contextValue}>
      {children}
    </UploadContext.Provider>
  );
}
