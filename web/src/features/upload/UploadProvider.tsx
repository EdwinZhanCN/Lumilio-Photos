import React, { ReactNode, useReducer, useCallback, useMemo } from "react";
import { useUploadProcess } from "./hooks/useUploadProcess";
import { uploadReducer, initialState } from "./reducers";
import { useMessage } from "@/features/notifications";
import { UploadContext } from "./upload.type.ts";
import { useI18n } from "@/lib/i18n"; // Import useI18n

/**
 * Provider component that manages upload state and operations.
 *
 * This component wraps the application with upload functionality, providing:
 * - File upload state management using useReducer
 * - Drag and drop event handling
 * - File processing and upload orchestration
 * - Progress tracking and error handling
 * - Integration with settings and internationalization
 *
 * @param props - Component props
 * @param props.children - Child components that will have access to upload context
 *
 * @example
 * ```typescript
 * function App() {
 *   return (
 *     <UploadProvider>
 *       <YourAppComponents />
 *     </UploadProvider>
 *   );
 * }
 *
 * function UploadComponent() {
 *   const { addFiles, uploadFiles, isProcessing } = useUploadContext();
 *
 *   const handleUpload = async (files: File[]) => {
 *     await addFiles(files);
 *     await uploadFiles();
 *   };
 *
 *   return <YourUploadUI onUpload={handleUpload} />;
 * }
 * ```
 */
export function UploadProvider({ children }: { children: ReactNode }) {
  const { t } = useI18n(); // Initialize useI18n
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

  const handleDrop = useCallback((e: React.DragEvent, handleFiles?: (files: FileList) => void) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: false });
    const droppedFiles = e.dataTransfer?.files;
    if (handleFiles && droppedFiles?.length) {
      handleFiles(droppedFiles);
    }
  }, []);

  /**
   * Add files to the upload queue
   * @param files - Files to add
   */
  const addFiles = useCallback(async (files: File[]) => {
    if (files.length === 0) return;

    // Add files with empty preview strings initially
    dispatch({
      type: "ADD_FILES",
      payload: {
        files,
        previews: files.map(() => ""),
      },
    });
  }, []);

  const clearFiles = useCallback(() => {
    dispatch({ type: "CLEAR_FILES" });
  }, []);

  const uploadFiles = useCallback(async () => {
    if (!state.files.length) {
      showMessage("info", t("upload.UploadProvider.no_files_selected_for_upload_message"));
      return;
    }

    try {
      const result = await uploadProcess.processFiles(state.files);
      if (result.failed.length > 0) {
        dispatch({ type: "RETAIN_FILES", payload: result.failed.map((failure) => failure.file) });
      } else {
        dispatch({ type: "CLEAR_FILES" });
      }
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : t("upload.UploadProvider.upload_failed_generic");
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
      isProcessing: uploadProcess.isGeneratingHashCodes || uploadProcess.isUploading,
      uploadProgress: uploadProcess.uploadProgress,
      hashcodeProgress: uploadProcess.hashcodeProgress,
      isGeneratingHashCodes: uploadProcess.isGeneratingHashCodes,
      isGeneratingPreviews: false,
      previewProgress: null,
      fileProgress: uploadProcess.fileProgress,
      maxPreviewCount: 0,
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
    ],
  );

  return <UploadContext.Provider value={contextValue}>{children}</UploadContext.Provider>;
}
