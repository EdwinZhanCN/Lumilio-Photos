import { DragEvent, RefObject, Dispatch, createContext } from "react";

/**
 * State for preview files, which include thumbnails.
 */
export interface PreviewUploadState {
  files: File[];
  previews: string[];
  count: number;
}

/**
 * State for batch files, which are large files without previews.
 */
export interface BatchUploadState {
  files: File[];
  count: number;
}

/**
 * Combined state for the entire upload feature.
 */
export interface UploadState {
  preview: PreviewUploadState;
  batch: BatchUploadState;
  totalFilesCount: number;
  isDragging: boolean;
  readonly maxPreviewFiles: number;
  readonly maxBatchFiles: number;
}

/**
 * Actions supported by the upload reducer.
 */
export type UploadAction =
  | { type: "SET_DRAGGING"; payload: boolean }
  | {
      type: "SET_PREVIEW_FILES";
      payload: { files: File[]; previews: string[] };
    }
  | {
      type: "UPDATE_PREVIEW_URLS";
      payload: { startIndex: number; urls: string[] };
    }
  | { type: "SET_BATCH_FILES"; payload: { files: File[] } }
  | { type: "CLEAR_PREVIEW_FILES" }
  | { type: "CLEAR_BATCH_FILES" }
  | { type: "CLEAR_ALL_FILES" };

/**
 * The value provided by the UploadContext.
 */
export interface UploadContextValue {
  state: UploadState;
  dispatch: Dispatch<UploadAction>;
  handleDragOver: (e: DragEvent) => void;
  handleDragLeave: (e: DragEvent) => void;
  handleDrop: (e: DragEvent, handleFiles?: (files: FileList) => void) => void;
  clearPreviewFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  clearBatchFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  clearAllFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  uploadPreviewFiles: () => Promise<void>;
  uploadBatchFiles: () => Promise<void>;
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
