/**
 * @fileoverview Type definitions for image export and download functionality
 *
 * @author Edwin Zhan
 * @since 1.0.0
 */

export interface ExportOptions {
  format: "jpeg" | "png" | "webp" | "original";
  quality: number; // 0.1 to 1.0 for lossy formats
  maxWidth?: number;
  maxHeight?: number;
  filename?: string;
}

export interface ExportRequest {
  imageUrl: string;
  options: ExportOptions;
}

export interface ExportResponse {
  status: "complete" | "error";
  blob?: Blob;
  filename?: string;
  error?: string;
}

export interface ExportWorkerMessage {
  type: "ABORT" | "INIT_WASM" | "EXPORT_IMAGE";
  data?: ExportRequest;
}

export interface ExportWorkerResponse {
  type: "WASM_READY" | "PROGRESS" | "EXPORT_COMPLETE" | "ERROR";
  result?: {
    blob: Blob;
    filename: string;
  };
  payload?: {
    processed: number;
    error?: string;
  };
  error?: string;
}

export interface ExportProgress {
  processed: number;
  total: number;
  currentFile?: string;
  error?: string;
}

export interface ExportResult {
  status: "complete" | "error";
  blob?: Blob;
  filename?: string;
  error?: string;
}

/**
 * Supported export formats with their characteristics
 */
export const EXPORT_FORMATS = {
  original: {
    label: "Original",
    extension: "",
    mimeType: "",
    supportsQuality: false,
    description: "Download the original file without any processing",
  },
  jpeg: {
    label: "JPEG",
    extension: "jpg",
    mimeType: "image/jpeg",
    supportsQuality: true,
    description: "Compressed format, good for photos with many colors",
  },
  png: {
    label: "PNG",
    extension: "png",
    mimeType: "image/png",
    supportsQuality: false,
    description: "Lossless format, good for images with transparency",
  },
  webp: {
    label: "WebP",
    extension: "webp",
    mimeType: "image/webp",
    supportsQuality: true,
    description: "Modern format with excellent compression",
  },
} as const;

/**
 * Common export presets for quick selection
 */
export const EXPORT_PRESETS = {
  web_small: {
    label: "Web (Small)",
    options: {
      format: "jpeg" as const,
      quality: 0.8,
      maxWidth: 800,
      maxHeight: 600,
    },
  },
  web_medium: {
    label: "Web (Medium)",
    options: {
      format: "jpeg" as const,
      quality: 0.85,
      maxWidth: 1200,
      maxHeight: 900,
    },
  },
  web_large: {
    label: "Web (Large)",
    options: {
      format: "jpeg" as const,
      quality: 0.9,
      maxWidth: 1920,
      maxHeight: 1440,
    },
  },
  print_quality: {
    label: "Print Quality",
    options: {
      format: "png" as const,
      quality: 1.0,
    },
  },
  social_media: {
    label: "Social Media",
    options: {
      format: "jpeg" as const,
      quality: 0.85,
      maxWidth: 1080,
      maxHeight: 1080,
    },
  },
} as const;

export type ExportPresetKey = keyof typeof EXPORT_PRESETS;
export type ExportFormat = keyof typeof EXPORT_FORMATS;
