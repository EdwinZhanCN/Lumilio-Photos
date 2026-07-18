import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { getToken } from "@/lib/http-commons/auth";

export function filenameFromContentDisposition(
  contentDisposition: string | null,
): string | undefined {
  if (!contentDisposition) return undefined;
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) return decodeURIComponent(utf8Match[1]);
  return contentDisposition.match(/filename="?([^"]+)"?/i)?.[1];
}

export function triggerBrowserDownload(blob: Blob, filename: string): void {
  const blobUrl = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = blobUrl;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(blobUrl);
}

async function downloadArchive(assetIds: string[]): Promise<void> {
  const headers = new Headers({ "Content-Type": "application/json" });
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(assetUrls.getBulkDownloadUrl(), {
    method: "POST",
    headers,
    body: JSON.stringify({ asset_ids: assetIds }),
  });
  if (!response.ok) throw new Error(`Bulk download failed with ${response.status}`);

  triggerBrowserDownload(
    await response.blob(),
    filenameFromContentDisposition(response.headers.get("content-disposition")) ??
      "lumilio-assets.zip",
  );
}

async function downloadIndividualAssets(assetIds: string[], assets?: Asset[]): Promise<void> {
  for (const assetId of assetIds) {
    try {
      const response = await fetch(assetUrls.getOriginalFileUrl(assetId));
      if (!response.ok) throw new Error(`Asset download failed with ${response.status}`);

      const asset = assets?.find((candidate) => candidate.asset_id === assetId);
      const filename =
        asset?.original_filename ??
        filenameFromContentDisposition(response.headers.get("content-disposition")) ??
        `asset-${assetId}`;
      triggerBrowserDownload(await response.blob(), filename);
    } catch (error) {
      console.error(`Failed to download asset ${assetId}:`, error);
    }

    await new Promise((resolve) => window.setTimeout(resolve, 300));
  }
}

export async function downloadAssets(assetIds: string[], assets?: Asset[]): Promise<void> {
  if (assetIds.length === 0) return;
  if (assetIds.length > 10) return downloadArchive(assetIds);
  return downloadIndividualAssets(assetIds, assets);
}
