/**
 * @fileoverview Upload Context Provider for managing file upload UI state and operations.
 *
 * This module provides a React context for handling the UI aspects of file uploads,
 * such as file selection, preview management, and drag & drop functionality.
 * It orchestrates the upload process by using a dedicated processing hook.
 *
 * This provider must be a child of WorkerProvider to function correctly.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */
import {
  createContext,
  useCallback,
  useContext,
  useReducer,
  useMemo,
  ReactNode,
  DragEvent,
  RefObject,
  Dispatch,
} from "react";
import { useUploadProcess } from "@/hooks/api-hooks/useUploadProcess";
import { useMessage } from "@/hooks/util-hooks/useMessage";

// --- State and Action Definitions ---

interface UploadState {
  /** Array of preview files (with thumbnails) */
  previewFiles: File[];
  /** Array of preview image URLs corresponding to preview files */
  previewPreviews: string[];
  /** Array of batch files (large files without previews) */
  batchFiles: File[];
  /** Total count of preview files */
  previewFilesCount: number;
  /** Total count of batch files */
  batchFilesCount: number;
  /** Total count of all files */
  totalFilesCount: number;
  /** Indicates if a user is dragging files over the drop zone */
  isDragging: boolean;
  /** Maximum number of files to show in the preview list */
  readonly maxPreviewFiles: number;
  /** Maximum number of batch files allowed */
  readonly maxBatchFiles: number;
}

type UploadAction =
  | { type: "SET_DRAGGING"; payload: boolean }
  | {
      type: "SET_PREVIEW_FILES";
      payload: { files: File[]; previews: string[] };
    }
  | { type: "SET_BATCH_FILES"; payload: { files: File[] } }
  | { type: "CLEAR_PREVIEW_FILES" }
  | { type: "CLEAR_BATCH_FILES" }
  | { type: "CLEAR_ALL_FILES" };

const initialState: UploadState = {
  previewFiles: [],
  previewPreviews: [],
  batchFiles: [],
  previewFilesCount: 0,
  batchFilesCount: 0,
  totalFilesCount: 0,
  isDragging: false,
  maxPreviewFiles: 30,
  maxBatchFiles: 50,
};

const uploadReducer = (
  state: UploadState,
  action: UploadAction,
): UploadState => {
  switch (action.type) {
    case "SET_DRAGGING":
      return { ...state, isDragging: action.payload };
    case "SET_PREVIEW_FILES":
      // Clean up old preview URLs before setting new ones
      state.previewPreviews.forEach((url) => URL.revokeObjectURL(url));
      const newPreviewCount = action.payload.files.length;
      return {
        ...state,
        previewFiles: action.payload.files,
        previewPreviews: action.payload.previews,
        previewFilesCount: newPreviewCount,
        totalFilesCount: newPreviewCount + state.batchFilesCount,
      };
    case "SET_BATCH_FILES":
      const newBatchCount = action.payload.files.length;
      return {
        ...state,
        batchFiles: action.payload.files,
        batchFilesCount: newBatchCount,
        totalFilesCount: state.previewFilesCount + newBatchCount,
      };
    case "CLEAR_PREVIEW_FILES":
      state.previewPreviews.forEach((url) => URL.revokeObjectURL(url));
      return {
        ...state,
        previewFiles: [],
        previewPreviews: [],
        previewFilesCount: 0,
        totalFilesCount: state.batchFilesCount,
      };
    case "CLEAR_BATCH_FILES":
      return {
        ...state,
        batchFiles: [],
        batchFilesCount: 0,
        totalFilesCount: state.previewFilesCount,
      };
    case "CLEAR_ALL_FILES":
      state.previewPreviews.forEach((url) => URL.revokeObjectURL(url));
      return { ...initialState }; // Reset to initial state
    default:
      return state;
  }
};

// --- Context Definition ---

interface UploadContextValue {
  state: UploadState;
  dispatch: Dispatch<UploadAction>;
  handleDragOver: (e: DragEvent) => void;
  handleDragLeave: (e: DragEvent) => void;
  handleDrop: (e: DragEvent, handleFiles?: (files: FileList) => void) => void;
  clearPreviewFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  clearBatchFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  clearAllFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  /**
   * Initiates upload for preview files only.
   */
  uploadPreviewFiles: () => Promise<void>;
  /**
   * Initiates upload for batch files only.
   */
  uploadBatchFiles: () => Promise<void>;
  /**
   * Initiates upload for all selected files (both preview and batch).
   */
  uploadAllFiles: () => Promise<void>;
  isProcessing: boolean;
  resetUploadStatus: () => void;
  uploadProgress: number;
  hashcodeProgress: {
    numberProcessed?: number;
    total?: number;
    error?: string;
  } | null;
  isGeneratingHashCodes: boolean;
}

export const UploadContext = createContext<UploadContextValue | undefined>(
  undefined,
);

// --- Provider Component ---

interface UploadProviderProps {
  children: ReactNode;
}

/**
 * Main provider that manages upload UI state and coordinates the upload process.
 */
export default function UploadProvider({ children }: UploadProviderProps) {
  const [state, dispatch] = useReducer(uploadReducer, initialState);
  const showMessage = useMessage();
  // The useUploadProcess hook will be refactored next to use our new hooks
  const uploadProcess = useUploadProcess();

  const handleDragOver = useCallback((e: DragEvent) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: true });
  }, []);

  const handleDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: false });
  }, []);

  const handleDrop = useCallback(
    (e: DragEvent, handleFiles?: (files: FileList) => void) => {
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
    (fileInputRef: RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_PREVIEW_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  const clearBatchFiles = useCallback(
    (fileInputRef: RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_BATCH_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  const clearAllFiles = useCallback(
    (fileInputRef: RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_ALL_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  const uploadPreviewFiles = useCallback(async () => {
    if (!state.previewFiles.length) {
      showMessage("info", "No preview files selected for upload.");
      return;
    }
    try {
      await uploadProcess.processFiles(state.previewFiles);
      dispatch({ type: "CLEAR_PREVIEW_FILES" });
    } catch (error: any) {
      showMessage("error", `Preview upload failed: ${error.message}`);
    }
  }, [state.previewFiles, uploadProcess, showMessage]);

  const uploadBatchFiles = useCallback(async () => {
    if (!state.batchFiles.length) {
      showMessage("info", "No batch files selected for upload.");
      return;
    }
    try {
      await uploadProcess.processFiles(state.batchFiles);
      dispatch({ type: "CLEAR_BATCH_FILES" });
    } catch (error: any) {
      showMessage("error", `Batch upload failed: ${error.message}`);
    }
  }, [state.batchFiles, uploadProcess, showMessage]);

  const uploadAllFiles = useCallback(async () => {
    const allFiles = [...state.previewFiles, ...state.batchFiles];
    if (!allFiles.length) {
      showMessage("info", "No files selected for upload.");
      return;
    }
    try {
      await uploadProcess.processFiles(allFiles);
      dispatch({ type: "CLEAR_ALL_FILES" });
    } catch (error: any) {
      showMessage("error", `Upload failed: ${error.message}`);
    }
  }, [state.previewFiles, state.batchFiles, uploadProcess, showMessage]);

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

// --- Custom Hook ---

/**
 * Custom hook for consuming the upload context.
 * Provides type-safe access to upload state and operations.
 * @throws Error if used outside of UploadProvider
 */
export function useUploadContext() {
  const context = useContext(UploadContext);
  if (!context) {
    throw new Error("useUploadContext must be used within an UploadProvider");
  }
  return context;
}
