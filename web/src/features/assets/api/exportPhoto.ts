import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { readProblemResponse } from "@/lib/http-commons/problem";
import {
  exportDimensions,
  exportBaseName,
  type PhotoExportSettings,
  type PhotoExportArtifact,
} from "@/lib/photo-export/model";

async function fetchBlob(url: string, signal?: AbortSignal): Promise<Blob> {
  const response = await fetch(url, { credentials: "include", signal });
  if (!response.ok) throw await readProblemResponse(response);
  return response.blob();
}
export function downloadOriginalPhoto(asset: Asset, signal?: AbortSignal): Promise<Blob> {
  if (!asset.asset_id) throw new Error("Missing asset ID");
  return fetchBlob(assetUrls.getOriginalFileUrl(asset.asset_id), signal);
}
export async function exportPhoto(
  asset: Asset,
  settings: PhotoExportSettings,
  signal: AbortSignal,
): Promise<PhotoExportArtifact> {
  if (!asset.asset_id) throw new Error("Missing asset ID");
  // Equal bounds express a long edge independently of EXIF orientation.
  const size = exportDimensions(asset.width ?? 0, asset.height ?? 0, settings.sizeMode);
  const maxSize =
    settings.sizeMode.kind === "original"
      ? undefined
      : size
        ? Math.max(size.width, size.height)
        : settings.sizeMode.kind === "longEdge"
          ? settings.sizeMode.longEdge
          : undefined;
  const blob = await fetchBlob(
    assetUrls.getExportUrl(asset.asset_id, {
      format: settings.format,
      quality: Math.round(settings.quality * 100),
      maxWidth: maxSize,
      maxHeight: maxSize,
      filename: exportBaseName(settings.filename),
    }),
    signal,
  );
  return { blob };
}
