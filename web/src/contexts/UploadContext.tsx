/**
 * @fileoverview Upload Context Provider for managing file upload state and operations
 *
 * This module provides a React context for handling file uploads, drag & drop functionality,
 * WASM worker integration, and upload progress tracking. It centralizes all upload-related
 * state management and provides a clean API for components to interact with upload operations.
 *
 * @author Edwin Zhan
 * @since 1.0.0
 *
 * @example
 * ```tsx
 * // Wrap your app with the UploadProvider
 * function App() {
 *   return (
 *     <UploadProvider>
 *       <YourComponents />
 *     </UploadProvider>
 *   );
 * }
 *
 * // Use the context in your components
 * function FileUploadComponent() {
 *   const { state, BatchUpload, clearFiles } = useUploadContext();
 *
 *   const handleFileSelect = async (files: FileList) => {
 *     await BatchUpload(files);
 *   };
 *
 *   return (
 *     <div>
 *       <p>Files selected: {state.filesCount}</p>
 *       <button onClick={() => clearFiles(fileInputRef)}>Clear</button>
 *     </div>
 *   );
 * }
 * ```
 */
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  ReactNode,
  DragEvent,
  RefObject,
  useReducer,
  useMemo,
  Dispatch,
} from "react";
import { WasmWorkerClient } from "@/workers/workerClient";
import { useUploadProcess } from "@/hooks/api-hooks/useUploadProcess";
import { useMessage } from "@/hooks/util-hooks/useMessage";

/**
 * **Upload State Interface**
 *
 * Defines the complete state structure for file upload operations including
 * selected files, preview generation, drag & drop status, and WASM readiness.
 *
 * @interface UploadState
 * @since 1.0.0
 *
 */
interface UploadState {
  /** Array of selected files ready for upload */
  files: File[];

  /**
   * Array of preview image URLs corresponding to selected files.
   * `null` entries indicate files without preview capability.
   *
   * @remarks URLs are created using `URL.createObjectURL()` and should be
   * revoked with `URL.revokeObjectURL()` to prevent memory leaks.
   */
  previews: (string | null)[];

  /** Total number of selected files */
  filesCount: number;

  /**
   * Indicates whether user is currently dragging files over the drop zone.
   * Used for visual feedback during drag & drop operations.
   */
  isDragging: boolean;

  /**
   * Indicates whether WASM modules are initialized and ready for use.
   * Must be `true` before upload operations can begin.
   */
  wasmReady: boolean;

  /**
   * Maximum number of preview files to display in the UI.
   * Prevents performance issues with large file selections.
   *
   * @readonly
   * @default 30
   */
  readonly maxPreviewFiles: number;
}

/**
 * **Upload Action Union Type**
 *
 * Defines all possible actions that can be dispatched to modify upload state.
 * Uses discriminated union pattern for type safety and predictable state updates.
 *
 * @since 1.0.0
 *
 */
type UploadAction =
  | {
      /** Sets the drag & drop state */
      type: "SET_DRAGGING";
      payload: boolean;
    }
  | {
      /** Updates selected files and their previews simultaneously */
      type: "SET_FILES";
      payload: { files: File[]; previews: (string | null)[] };
    }
  | {
      /** Sets WASM module readiness state */
      type: "SET_WASM_READY";
      payload: boolean;
    }
  | {
      /** Clears all selected files and previews */
      type: "CLEAR_FILES";
    };

/**
 * **Initial Upload State**
 *
 * Default state values used when initializing the upload context.
 *
 * @internal
 */
const initialState: UploadState = {
  files: [],
  previews: [],
  filesCount: 0,
  isDragging: false,
  wasmReady: false,
  maxPreviewFiles: 30,
};

/**
 * **Upload State Reducer**
 *
 * Pure function that handles all upload state transitions based on dispatched actions.
 * Includes automatic cleanup of preview URLs to prevent memory leaks.
 *
 * @param state - Current upload state
 * @param action - Action to process
 * @returns New upload state
 *
 * @internal
 *
 */
const uploadReducer = (
  state: UploadState,
  action: UploadAction,
): UploadState => {
  switch (action.type) {
    case "SET_DRAGGING":
      return { ...state, isDragging: action.payload };
    case "SET_FILES":
      // Clean up existing preview URLs to prevent memory leaks
      state.previews.forEach((url) => url && URL.revokeObjectURL(url));
      return {
        ...state,
        files: action.payload.files,
        previews: action.payload.previews,
        filesCount: action.payload.files.length,
      };
    case "SET_WASM_READY":
      return { ...state, wasmReady: action.payload };
    case "CLEAR_FILES":
      // Clean up preview URLs before clearing
      state.previews.forEach((url) => url && URL.revokeObjectURL(url));
      return { ...state, files: [], previews: [], filesCount: 0 };
    default:
      return state;
  }
};

/**
 * **Upload Context Value Interface**
 *
 * Defines the complete API provided by the UploadContext to consuming components.
 * Includes state, actions, event handlers, and upload operations.
 *
 * @interface UploadContextValue
 * @since 1.0.0
 *
 */
interface UploadContextValue {
  state: UploadState;

  /**
   * Dispatch function for triggering state changes.
   *
   * @see {@link UploadAction} for available actions
   */
  dispatch: Dispatch<UploadAction>;

  /**
   * Reference to the WASM worker client instance.
   * Used for thumbnail generation and hash computation.
   *
   * @see {@link WasmWorkerClient}
   */
  workerClientRef: React.RefObject<WasmWorkerClient | null>;
  handleDragOver: (e: DragEvent) => void;
  handleDragLeave: (e: DragEvent) => void;
  handleDrop: (e: DragEvent, handleFiles?: (files: FileList) => void) => void;
  clearFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;

  /**
   * **Batch Upload Function**
   *
   * Initiates the batch upload process for selected files. Handles thumbnail
   * generation, hash computation, and server upload in sequence.
   *
   * @param selectedFiles - FileList containing files to upload
   * @throws Will throw an error if WASM is not ready or upload fails
   *
   */
  BatchUpload: (selectedFiles: FileList) => Promise<void>;

  isProcessing: boolean;
  resetUploadStatus: () => void;
  uploadProgress: number;

  /**
   * **Hash Code Generation Progress**
   *
   * Detailed progress information for hash code generation phase.
   * Includes processing counts, error states, and failure points.
   *
   * @example
   * ```tsx
   * {hashcodeProgress && (
   *   <div>
   *     <p>Processing: {hashcodeProgress.numberProcessed}/{hashcodeProgress.total}</p>
   *     {hashcodeProgress.error && (
   *       <p>Error: {hashcodeProgress.error}</p>
   *     )}
   *   </div>
   * )}
   * ```
   */
  hashcodeProgress: {
    numberProcessed?: number;
    total?: number;
    error?: string;
    failedAt?: number;
  } | null;

  isGeneratingHashCodes: boolean;
}

/**
 * **Upload Provider Props**
 *
 * Props interface for the UploadProvider component.
 *
 * @interface UploadProviderProps
 */
interface UploadProviderProps {
  /** Child components that will have access to the upload context */
  children: ReactNode;
}

/**
 * **Upload Context**
 *
 * React context for sharing upload state and operations across components.
 *
 * @since 1.0.0
 * @see {@link useUploadContext} for consuming the context
 */
export const UploadContext = createContext<UploadContextValue | undefined>(
  undefined,
);

/**
 * **Upload Provider Component**
 *
 * Main provider component that manages upload state and provides context to child components.
 * Handles WASM initialization, state management, and coordinates upload operations.
 *
 * @param props - Provider props containing children
 * @returns JSX element wrapping children with upload context
 *
 * @example
 * ```tsx
 * // At the root of your application
 * function App() {
 *   return (
 *     <UploadProvider>
 *       <Header />
 *       <MainContent />
 *       <Footer />
 *     </UploadProvider>
 *   );
 * }
 *
 * // In any child component
 * function FileUploadZone() {
 *   const { state, BatchUpload } = useUploadContext();
 *   // Component implementation...
 * }
 * ```
 *
 * @since 1.0.0
 */
export default function UploadProvider({ children }: UploadProviderProps) {
  const [state, dispatch] = useReducer(uploadReducer, initialState);
  const { wasmReady, previews } = state;

  const showMessage = useMessage();
  const workerClientRef = useRef<WasmWorkerClient | null>(null);
  const uploadProcess = useUploadProcess(workerClientRef, wasmReady);

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

  const clearFiles = useCallback(
    (fileInputRef: RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  /**
   * Initialize WASM modules when component mounts.
   * Sets up thumbnail generation and hash computation capabilities.
   *
   * @internal
   */
  useEffect(() => {
    if (!workerClientRef.current) {
      workerClientRef.current = new WasmWorkerClient();
    }
    const initWasm = async () => {
      try {
        await workerClientRef.current?.initGenThumbnailWASM();
        await workerClientRef.current?.initGenHashWASM();
        dispatch({ type: "SET_WASM_READY", payload: true });
        console.log("WASM module initialized successfully");
      } catch (error) {
        console.error("Failed to initialize WASM:", error);
      }
    };
    initWasm();

    // Cleanup preview URLs and terminate workers on unmount
    return () => {
      previews.forEach((url) => url && URL.revokeObjectURL(url));
      if (workerClientRef.current) {
        workerClientRef.current.terminateGenerateThumbnailWorker();
        workerClientRef.current.terminateGenerateHashWorker();
        workerClientRef.current.terminateGenerateBorderWorker();
        // If there's a worker for EXIF extraction, terminate it as well.
        // Assuming the client has a method for it, e.g., terminateExtractExifWorker()
        // workerClientRef.current.terminateExtractExifWorker();
      }
    };
  }, [previews]); // previews as dependency to ensure latest preview list is used during cleanup

  const BatchUpload = useCallback(
    async (selectedFiles: FileList) => {
      if (!wasmReady || !selectedFiles.length) {
        showMessage(
          "error",
          "Cannot upload: WASM not initialized or no files selected",
        );
        return;
      }
      try {
        await uploadProcess.processFiles(selectedFiles);
      } catch (error: any) {
        showMessage("error", `Upload process failed: ${error.message}`);
      }
      uploadProcess.resetStatus();
    },
    [wasmReady, uploadProcess, showMessage],
  );

  const contextValue = useMemo(
    () => ({
      state, // The current state of upload process
      dispatch, // dispatch action, flexible
      workerClientRef, // instance of wasm worker client
      handleDragOver, // drag&drop state
      handleDragLeave, // drag&drop state
      handleDrop, // drag&drop state
      clearFiles, // clear all files in the upload array buffer
      BatchUpload, // the function to batch upload files
      isProcessing:
        uploadProcess.isGeneratingHashCodes || uploadProcess.isUploading, // the state shows is generating hashcodes or upload
      resetUploadStatus: uploadProcess.resetStatus, // reset upload process
      uploadProgress: uploadProcess.uploadProgress, // upload progress
      hashcodeProgress: uploadProcess.hashcodeProgress, // hashcode generation progress
      isGeneratingHashCodes: uploadProcess.isGeneratingHashCodes, // another specific state to indicates if it is generating hashcodes.
    }),
    [
      state,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      clearFiles,
      BatchUpload,
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

/**
 * **Upload Context Hook**
 *
 * Custom hook for consuming the upload context. Provides type-safe access
 * to upload state and operations with automatic error handling.
 *
 * @returns Upload context value with all state and methods
 * @throws Error if used outside of UploadProvider
 *
 * @since 1.0.0
 * @see {@link UploadProvider} for the context provider
 * @see {@link UploadContextValue} for the complete API reference
 */
export function useUploadContext() {
  const context = useContext(UploadContext);
  if (!context) {
    throw new Error("useUploadContext must be used within an UploadProvider");
  }
  return context;
}
