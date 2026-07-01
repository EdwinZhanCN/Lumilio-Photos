export interface ToolRunContext {
  inputFile: File;
  signal: AbortSignal;
}

export interface ToolRunHelpers {
  reportProgress?: (processed: number, total: number) => void;
}

export interface ToolRunResult {
  bytes: Uint8Array;
  mimeType: string;
  fileName: string;
}

export type ToolRunner = (
  ctx: ToolRunContext,
  params: Record<string, unknown>,
  helpers?: ToolRunHelpers,
) => Promise<ToolRunResult>;

export const SUPPORTED_IMAGE_MIME_TYPES = ["image/jpeg", "image/png", "image/webp"] as const;

export type SupportedImageMimeType = (typeof SUPPORTED_IMAGE_MIME_TYPES)[number];
