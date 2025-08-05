import React, { ReactNode, useReducer, useCallback, useMemo } from "react";
import { useUploadProcess } from "@/hooks/api-hooks/useUploadProcess";
import { uploadReducer, initialState } from "./reducers";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { UploadContext } from "./types";

export function UploadProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(uploadReducer, initialState);
  const showMessage = useMessage();
  const uploadProcess = useUploadProcess();

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

  const clearPreviewFiles = useCallback(
    (fileInputRef: React.RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_PREVIEW_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  const clearBatchFiles = useCallback(
    (fileInputRef: React.RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_BATCH_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  const clearAllFiles = useCallback(
    (fileInputRef: React.RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_ALL_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  const uploadPreviewFiles = useCallback(async () => {
    if (!state.preview.files.length) {
      showMessage("info", "No preview files selected for upload.");
      return;
    }
    try {
      await uploadProcess.processFiles(state.preview.files);
      dispatch({ type: "CLEAR_PREVIEW_FILES" });
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Preview upload failed";
      showMessage("error", errorMessage);
    }
  }, [state.preview.files, uploadProcess, showMessage]);

  const uploadBatchFiles = useCallback(async () => {
    if (!state.batch.files.length) {
      showMessage("info", "No batch files selected for upload.");
      return;
    }
    try {
      await uploadProcess.processFiles(state.batch.files);
      dispatch({ type: "CLEAR_BATCH_FILES" });
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Batch upload failed";
      showMessage("error", errorMessage);
    }
  }, [state.batch.files, uploadProcess, showMessage]);

  const uploadAllFiles = useCallback(async () => {
    const allFiles = [...state.preview.files, ...state.batch.files];
    if (!allFiles.length) {
      showMessage("info", "No files selected for upload.");
      return;
    }
    try {
      await uploadProcess.processFiles(allFiles);
      dispatch({ type: "CLEAR_ALL_FILES" });
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Upload failed";
      showMessage("error", errorMessage);
    }
  }, [state.preview.files, state.batch.files, uploadProcess, showMessage]);

  const contextValue = useMemo(
    () => ({
      state,
      dispatch,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      clearPreviewFiles,
      clearBatchFiles,
      clearAllFiles,
      uploadPreviewFiles,
      uploadBatchFiles,
      uploadAllFiles,
      isProcessing:
        uploadProcess.isGeneratingHashCodes || uploadProcess.isUploading,
      resetUploadStatus: uploadProcess.resetStatus,
      uploadProgress: uploadProcess.uploadProgress,
      hashcodeProgress: uploadProcess.hashcodeProgress,
      isGeneratingHashCodes: uploadProcess.isGeneratingHashCodes,
    }),
    [
      state,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      clearPreviewFiles,
      clearBatchFiles,
      clearAllFiles,
      uploadPreviewFiles,
      uploadBatchFiles,
      uploadAllFiles,
      uploadProcess.isGeneratingHashCodes,
      uploadProcess.isUploading,
      uploadProcess.resetStatus,
      uploadProcess.uploadProgress,
      uploadProcess.hashcodeProgress,
    ],
  );

  return (
    <UploadContext.Provider value={contextValue}>
      {children}
    </UploadContext.Provider>
  );
}
