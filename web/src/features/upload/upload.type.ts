import { DragEvent, Dispatch, createContext } from "react";
import type { FileUploadProgress } from "./hooks/useUploadProcess";

/**
 * Single unified upload state interface.
 * 
 * Contains all the state information needed for file upload operations,
 * including file lists, previews, and drag-and-drop status.
 */
export interface UploadState {
  /** Array of selected files for upload */
  files: File[];
  /** Array of preview URLs - empty string indicates no preview generated */
  previews: string[];
  /** Whether files are currently being dragged over the drop zone */
  isDragging: boolean;
}

/**
 * Actions supported by the upload reducer.
 * 
 * These actions define all possible state mutations for the upload system,
 * following the Redux pattern for predictable state updates.
 */
export type UploadAction =
  | { type: "SET_DRAGGING"; payload: boolean }
  | { type: "ADD_FILES"; payload: { files: File[]; previews: string[] } }
  | {
    type: "UPDATE_PREVIEW_URLS";
    payload: { startIndex: number; urls: string[] };
  }
  | { type: "CLEAR_FILES" };

/**
 * The value provided by the UploadContext.
 * 
 * This interface defines the complete API available through the upload context,
 * including state, actions, drag-and-drop handlers, file operations, and status information.
 */
export interface UploadContextValue {
  state: UploadState;
  dispatch: Dispatch<UploadAction>;

  // Drag and drop handlers
  handleDragOver: (e: DragEvent) => void;
  handleDragLeave: (e: DragEvent) => void;
  handleDrop: (e: DragEvent, handleFiles?: (files: FileList) => void) => void;

  // File operations
  addFiles: (files: File[]) => Promise<void>;
  clearFiles: () => void;
  uploadFiles: () => Promise<void>;

  // Upload status
  isProcessing: boolean;
  uploadProgress: number;
  hashcodeProgress: {
    numberProcessed?: number;
    total?: number;
    error?: string;
  } | null;
  isGeneratingHashCodes: boolean;
  isGeneratingPreviews: boolean;
  previewProgress: {
    numberProcessed?: number;
    total?: number;
  } | null;
  fileProgress: FileUploadProgress[];

  // Settings
  maxPreviewCount: number;
  maxTotalFiles: number;
  previewCount: number; // Number of files with previews
}

export const UploadContext = createContext<UploadContextValue | undefined>(
  undefined,
);
