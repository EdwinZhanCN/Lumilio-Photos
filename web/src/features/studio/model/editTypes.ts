/** Serializable non-destructive adjustments shared by the editor and render worker. */
export type StudioEditAdjustments = {
  exposure: number;
  contrast: number;
  highlights: number;
  shadows: number;
  whites: number;
  blacks: number;
  temperature: number;
  tint: number;
  vibrance: number;
  saturation: number;
  clarity: number;
  sharpness: number;
  noiseReduction: number;
  rotation: number;
  flipHorizontal: boolean;
  flipVertical: boolean;
  crop: null | {
    x: number;
    y: number;
    width: number;
    height: number;
  };
};

export type LumilioSidecarSource = {
  original_filename: string;
  storage_path: string;
  mime_type: string;
  file_size: number;
  hash?: string | null;
  width?: number | null;
  height?: number | null;
};

export type LumilioSidecarV1 = {
  version: 1;
  asset_id: string;
  source: LumilioSidecarSource;
  adjustments: StudioEditAdjustments;
  updated_at: string;
};

export type AssetSidecarResponse = {
  asset_id: string;
  exists: boolean;
  sidecar: LumilioSidecarV1;
};

export const DEFAULT_STUDIO_ADJUSTMENTS: StudioEditAdjustments = {
  exposure: 0,
  contrast: 0,
  highlights: 0,
  shadows: 0,
  whites: 0,
  blacks: 0,
  temperature: 0,
  tint: 0,
  vibrance: 0,
  saturation: 0,
  clarity: 0,
  sharpness: 0,
  noiseReduction: 0,
  rotation: 0,
  flipHorizontal: false,
  flipVertical: false,
  crop: null,
};

export function normalizeStudioAdjustments(
  input: Partial<StudioEditAdjustments> | null | undefined,
): StudioEditAdjustments {
  return {
    ...DEFAULT_STUDIO_ADJUSTMENTS,
    ...input,
    flipHorizontal: Boolean(input?.flipHorizontal),
    flipVertical: Boolean(input?.flipVertical),
    crop: input?.crop ?? null,
  };
}
