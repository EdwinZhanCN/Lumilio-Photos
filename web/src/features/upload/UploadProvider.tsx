import React, { ReactNode, useReducer, useCallback, useMemo } from "react";
import { useUploadProcess } from "./hooks/useUploadProcess";
import { uploadReducer, initialState } from "./reducers";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { UploadContext } from "./upload.type.ts";
import { useSettingsContext } from "@/features/settings";
import { useI18n } from "@/lib/i18n"; // Import useI18n

export function UploadProvider({ children }: { children: ReactNode }) {
  const { t } = useI18n(); // Initialize useI18n
  const [state, dispatch] = useReducer(uploadReducer, initialState);
  const showMessage = useMessage();
  const uploadProcess = useUploadProcess();
  const { state: settings } = useSettingsContext();

  // Get settings with defaults
  const maxTotalFiles = settings.ui.upload?.max_total_files ?? 100;

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
   */
  const addFiles = useCallback(
    async (files: File[]) => {
      if (files.length === 0) return;

      // Check total files limit
      const availableSlots = maxTotalFiles - state.files.length;
      if (availableSlots <= 0) {
        showMessage("error", t('upload.UploadProvider.max_files_allowed', { count: maxTotalFiles }));
        return;
      }

      const filesToAdd = files.slice(0, availableSlots);
      if (files.length > availableSlots) {
        showMessage(
          "hint",
          t('upload.UploadProvider.files_exceeded_limit', { count: files.length - availableSlots }),
        );
      }

      // Add files with empty preview strings initially
      dispatch({
        type: "ADD_FILES",
        payload: {
          files: filesToAdd,
          previews: filesToAdd.map(() => ""),
        },
      });
    },
    [state.files.length, maxTotalFiles, showMessage, t],
  );

  const clearFiles = useCallback(() => {
    dispatch({ type: "CLEAR_FILES" });
  }, []);

  const uploadFiles = useCallback(async () => {
    if (!state.files.length) {
      showMessage("info", t('upload.UploadProvider.no_files_selected_for_upload_message'));
      return;
    }

    try {
      await uploadProcess.processFiles(state.files);
      dispatch({ type: "CLEAR_FILES" });
      showMessage("success", t('upload.UploadProvider.upload_completed_success'));
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : t('upload.UploadProvider.upload_failed_generic');
      showMessage("error", errorMessage);
    }
  }, [state.files, uploadProcess, showMessage, t]);

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
      isGeneratingPreviews: false,
      previewProgress: null,
      fileProgress: uploadProcess.fileProgress,
      maxPreviewCount: 0,
      maxTotalFiles,
      previewCount: 0,
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
      maxTotalFiles,
    ],
  );

  return (
    <UploadContext.Provider value={contextValue}>
      {children}
    </UploadContext.Provider>
  );
}
