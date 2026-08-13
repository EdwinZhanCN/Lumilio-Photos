export type VisualSearchErrorKind =
  | "embedding_missing"
  | "unavailable"
  | "invalid_image"
  | "generic";

const SEARCH_BY_IMAGE_EXTENSIONS = [
  ".jpg",
  ".jpeg",
  ".png",
  ".webp",
  ".gif",
  ".bmp",
  ".tiff",
  ".tif",
  ".heic",
  ".heif",
  ".cr2",
  ".cr3",
  ".nef",
  ".arw",
  ".dng",
  ".orf",
  ".rw2",
  ".pef",
  ".raf",
  ".mrw",
  ".srw",
  ".rwl",
  ".x3f",
] as const;

export const searchByImageAccept = ["image/*", ...SEARCH_BY_IMAGE_EXTENSIONS].join(",");

type SearchErrorBody = {
  error?: string;
  message?: string;
  code?: number;
  status?: number;
  reason?: string;
};

export function isSearchByImageFile(file: File): boolean {
  if (file.type.startsWith("image/")) return true;
  const dot = file.name.lastIndexOf(".");
  if (dot < 0) return false;
  const ext = file.name.slice(dot).toLowerCase();
  return (SEARCH_BY_IMAGE_EXTENSIONS as readonly string[]).includes(ext);
}

export function isAssetSearchActive(
  query?: string,
  similarAssetId?: string,
  fileQuery?: File | null,
): boolean {
  return Boolean(query?.trim() || similarAssetId?.trim() || fileQuery);
}

export function fileQueryIdentity(file: File | null | undefined): string {
  if (!file) return "";
  return `${file.name}:${file.size}:${file.lastModified}`;
}

export function classifySearchError(error: unknown): VisualSearchErrorKind {
  const body = unwrapSearchError(error);
  const reason = `${body.reason ?? ""} ${body.error ?? ""} ${body.message ?? ""}`.toLowerCase();
  const status = body.status ?? body.code;
  if (reason.includes("embedding_missing") || status === 409) return "embedding_missing";
  if (status === 503 || reason.includes("unavailable") || reason.includes("unreachable")) {
    return "unavailable";
  }
  if (status === 400 || reason.includes("invalid image") || reason.includes("unsupported")) {
    return "invalid_image";
  }
  return "generic";
}

export function throwSearchError(error: unknown, status?: number): never {
  const body = unwrapSearchError(error);
  const reason = body.error || body.message || "search failed";
  const err = new Error(reason) as Error & SearchErrorBody;
  err.error = body.error;
  err.message = reason;
  err.code = body.code;
  err.status = status ?? body.status ?? body.code;
  err.reason = body.error ?? body.reason;
  throw err;
}

function unwrapSearchError(error: unknown): SearchErrorBody {
  if (typeof error === "string") {
    try {
      return unwrapSearchError(JSON.parse(error));
    } catch {
      return { message: error, error: error, reason: error };
    }
  }
  if (error instanceof Error) {
    const extra = error as Error & SearchErrorBody;
    return {
      error: extra.error ?? extra.reason,
      message: extra.message,
      code: extra.code,
      status: extra.status,
      reason: extra.reason ?? extra.error,
    };
  }
  if (error && typeof error === "object") {
    return error as SearchErrorBody;
  }
  return { message: "search failed" };
}
