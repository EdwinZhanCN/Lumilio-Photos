// Lumilio-Photos/web/src/config/retryTasks.ts

/**
 * Configuration for asset retry/reprocess tasks.
 * Uses catalog pipeline stages as task identifiers. These are product-level
 * requests; River queue and macro-job names remain an implementation detail.
 */

export interface RetryTaskOption {
  key: string; // Catalog pipeline stage (canonical identifier)
  label: string;
  description: string;
  category: "metadata" | "media" | "ml";
  supportedAssetTypes: readonly AssetType[];
}

export type AssetType = "PHOTO" | "VIDEO" | "AUDIO";

/**
 * Available retry tasks using catalog stage identifiers.
 * Keep this in sync with asset_handler.go isValidReprocessStage.
 */
export const RETRY_TASK_OPTIONS: RetryTaskOption[] = [
  {
    key: "analyze",
    label: "Metadata Extraction",
    description: "Extract EXIF and file metadata",
    category: "metadata",
    supportedAssetTypes: ["PHOTO", "VIDEO", "AUDIO"],
  },
  {
    key: "derivatives",
    label: "Thumbnail Generation",
    description: "Generate thumbnails at multiple sizes",
    category: "media",
    supportedAssetTypes: ["PHOTO", "VIDEO"],
  },
  {
    key: "transcode",
    label: "Media Transcoding",
    description: "Transcode video/audio to web-optimized formats",
    category: "media",
    supportedAssetTypes: ["VIDEO", "AUDIO"],
  },
  {
    key: "enrich",
    label: "Image and Media Enrichment",
    description: "Run enabled semantic, OCR, face, and classification analysis",
    category: "ml",
    supportedAssetTypes: ["PHOTO", "VIDEO"],
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

export function normalizeAssetType(assetType: string | undefined): AssetType | undefined {
  switch (assetType) {
    case "PHOTO":
    case "VIDEO":
    case "AUDIO":
      return assetType;
    default:
      return undefined;
  }
}

export function isRetryTaskSupportedForAssetType(
  stage: string,
  assetType: string | undefined,
): boolean {
  const normalizedAssetType = normalizeAssetType(assetType);
  if (!normalizedAssetType) return false;

  const task = getRetryTaskOption(stage);
  return task?.supportedAssetTypes.includes(normalizedAssetType) ?? false;
}

export function getRetryTasksByCategoryForAssetType(
  assetType: string | undefined,
): typeof RETRY_TASKS_BY_CATEGORY {
  return {
    metadata: RETRY_TASKS_BY_CATEGORY.metadata.filter((task) =>
      isRetryTaskSupportedForAssetType(task.key, assetType),
    ),
    media: RETRY_TASKS_BY_CATEGORY.media.filter((task) =>
      isRetryTaskSupportedForAssetType(task.key, assetType),
    ),
    ml: RETRY_TASKS_BY_CATEGORY.ml.filter((task) =>
      isRetryTaskSupportedForAssetType(task.key, assetType),
    ),
  };
}

/**
 * Get task option by catalog stage.
 */
export function getRetryTaskOption(stage: string): RetryTaskOption | undefined {
  return RETRY_TASK_OPTIONS.find((t) => t.key === stage);
}

/**
 * Validate if a catalog stage is valid for retry.
 */
export function isValidRetryTask(stage: string): boolean {
  return RETRY_TASK_OPTIONS.some((t) => t.key === stage);
}
