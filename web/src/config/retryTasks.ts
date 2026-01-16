// Lumilio-Photos/web/src/config/retryTasks.ts

/**
 * Configuration for asset retry/reprocess tasks.
 * Uses queue names as task identifiers (bijection: queue name ↔ task type).
 * These must match the backend's validQueues in asset_handler.go.
 */

export interface RetryTaskOption {
  key: string; // Queue name (canonical identifier)
  label: string;
  description: string;
  category: "metadata" | "media" | "ml";
}

/**
 * Available retry tasks using queue names as identifiers.
 * Keep this in sync with asset_handler.go validQueues map.
 */
export const RETRY_TASK_OPTIONS: RetryTaskOption[] = [
  {
    key: "metadata_asset",
    label: "Metadata Extraction",
    description: "Extract EXIF and file metadata",
    category: "metadata",
  },
  {
    key: "thumbnail_asset",
    label: "Thumbnail Generation",
    description: "Generate thumbnails at multiple sizes",
    category: "media",
  },
  {
    key: "transcode_asset",
    label: "Media Transcoding",
    description: "Transcode video/audio to web-optimized formats",
    category: "media",
  },
  {
    key: "process_clip",
    label: "CLIP Embedding",
    description: "Generate AI embeddings for semantic search",
    category: "ml",
  },
  {
    key: "process_ocr",
    label: "OCR Text Extraction",
    description: "Extract text from images using OCR",
    category: "ml",
  },
  {
    key: "process_caption",
    label: "AI Image Captioning",
    description: "Generate AI-powered image descriptions",
    category: "ml",
  },
  {
    key: "process_face",
    label: "Face Detection",
    description: "Detect and recognize faces in images",
    category: "ml",
  },
];

/**
 * Group tasks by category for better UI organization.
 */
export const RETRY_TASKS_BY_CATEGORY = {
  metadata: RETRY_TASK_OPTIONS.filter((t) => t.category === "metadata"),
  media: RETRY_TASK_OPTIONS.filter((t) => t.category === "media"),
  ml: RETRY_TASK_OPTIONS.filter((t) => t.category === "ml"),
};

/**
 * Get task option by queue name.
 */
export function getRetryTaskOption(
  queueName: string,
): RetryTaskOption | undefined {
  return RETRY_TASK_OPTIONS.find((t) => t.key === queueName);
}

/**
 * Validate if a queue name is valid for retry.
 */
export function isValidRetryTask(queueName: string): boolean {
  return RETRY_TASK_OPTIONS.some((t) => t.key === queueName);
}
