import { getProblemType, normalizeProblem } from "@/lib/http-commons/problem";

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
  switch (getProblemType(error)) {
    case "https://lumilio.org/problems/media/image-embedding-missing":
      return "embedding_missing";
    case "https://lumilio.org/problems/lumen/image-semantic-analysis-unavailable":
    case "https://lumilio.org/problems/service/unavailable":
      return "unavailable";
    case "https://lumilio.org/problems/media/invalid-request":
      return "invalid_image";
    default:
      return "generic";
  }
}

export function throwSearchError(error: unknown): never {
  throw normalizeProblem(error);
}
