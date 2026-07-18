export { BorderPanel, defaultParams } from "./BorderPanel";
export { runBorderTransform } from "./borderRunner";
export { normalizeParams, DEFAULT_PARAMS, isExifBorderMode, EXIF_BORDER_MODES } from "./types";
export type { BorderMode, BorderParams } from "./types";

// EXIF + brand-logo helpers (main-thread consumers: editor + panel).
export { extractBorderExif, hasSufficientExif, cameraLabel, type BorderExif } from "./exifInfo";
export { matchBrandKey, brandDisplayName, rasterizeBrandLogo, type BrandKey } from "./logoAssets";
