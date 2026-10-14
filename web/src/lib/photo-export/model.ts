export type PhotoExportFormat = "jpeg" | "png" | "webp" | "avif";
export type PhotoExportSize =
  | { kind: "original" }
  | { kind: "percent"; percent: number }
  | { kind: "longEdge"; longEdge: number };
export type PhotoExportSettings = {
  format: PhotoExportFormat;
  quality: number;
  sizeMode: PhotoExportSize;
  filename: string;
};
export type PhotoExportArtifact = {
  blob: Blob;
  warnings?: string[];
};
export const PHOTO_EXPORT_FORMATS = ["jpeg", "png", "webp", "avif"] as const;
export const PHOTO_EXPORT_EXTENSIONS = {
  jpeg: "jpg",
  png: "png",
  webp: "webp",
  avif: "avif",
} as const;
export const PHOTO_EXPORT_MIME = {
  jpeg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
  avif: "image/avif",
} as const;

export function exportBaseName(filename: string): string {
  return (
    filename
      .replace(/\.(?:jpe?g|png|webp|avif)$/i, "")
      // Download names must exclude filesystem separators and ASCII control bytes.
      // eslint-disable-next-line no-control-regex
      .replace(/[<>:"/\\|?*\u0000-\u001f]/g, "_")
      .trim()
      .replace(/[. ]+$/, "") || "export"
  );
}
export function defaultPhotoExportSettings(filename = "photo"): PhotoExportSettings {
  return {
    format: "jpeg",
    quality: 0.92,
    sizeMode: { kind: "original" },
    filename: `${filename.replace(/\.[^.]+$/, "") || "photo"}-lumilio`,
  };
}
export function photoExportFilename(settings: PhotoExportSettings): string {
  return `${exportBaseName(settings.filename)}.${PHOTO_EXPORT_EXTENSIONS[settings.format]}`;
}
export function exportLongEdge(mode: PhotoExportSize, nativeLongEdge: number): number {
  const native = Math.max(1, Math.round(nativeLongEdge));
  const requested =
    mode.kind === "percent"
      ? (native * Math.min(100, Math.max(1, mode.percent))) / 100
      : mode.kind === "longEdge"
        ? mode.longEdge
        : native;
  return Math.min(native, Math.max(1, Math.round(Number.isFinite(requested) ? requested : native)));
}
export function exportDimensions(width: number, height: number, mode: PhotoExportSize) {
  if (!(width > 0 && height > 0)) return null;
  const scale = exportLongEdge(mode, Math.max(width, height)) / Math.max(width, height);
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  };
}
