import { DragEvent, Dispatch, createContext } from "react";
import type { FileUploadProgress } from "@/hooks/api-hooks/useUploadProcess";

/**
 * Single unified upload state
 */
export interface UploadState {
  files: File[];
  previews: string[]; // Empty string means no preview generated
  isDragging: boolean;
}

/**
 * Actions supported by the upload reducer.
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
 */
export interface UploadContextValue {
  state: UploadState;
  dispatch: Dispatch<UploadAction>;

  // Drag and drop handlers
  handleDragOver: (e: DragEvent) => void;
  handleDragLeave: (e: DragEvent) => void;
  handleDrop: (e: DragEvent, handleFiles?: (files: FileList) => void) => void;

  // File operations
  addFiles: (files: File[], generatePreviews: boolean) => Promise<void>;
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
