import { getMediaToken } from "@/lib/http-commons/auth.ts";

const baseURL = import.meta.env.VITE_API_URL ?? "";

const withMediaToken = (url: string): string => {
  const mediaToken = getMediaToken();
  if (!mediaToken) {
    return url;
  }
  const separator = url.includes("?") ? "&" : "?";
  return `${url}${separator}mt=${encodeURIComponent(mediaToken)}`;
};

/**
 * Asset media URL helpers for use outside the data layer.
 */
export const assetUrls = {
  getOriginalFileUrl(id: string): string {
    return withMediaToken(`${baseURL}/api/v1/assets/${id}/original`);
  },

  getBulkDownloadUrl(): string {
    return withMediaToken(`${baseURL}/api/v1/assets/download`);
  },

  /**
   * Server-side re-encode/download endpoint. The backend (libvips) transcodes
   * the original to the requested format/size and streams it as an attachment.
   */
  getExportUrl(
    id: string,
    params: {
      format: "jpeg" | "png" | "webp" | "avif";
      quality?: number; // 1-100
      maxWidth?: number;
      maxHeight?: number;
      filename?: string; // base name, without extension
    },
  ): string {
    const search = new URLSearchParams();
    search.set("format", params.format);
    if (params.quality != null) search.set("quality", String(params.quality));
    if (params.maxWidth != null) search.set("max_width", String(params.maxWidth));
    if (params.maxHeight != null) search.set("max_height", String(params.maxHeight));
    if (params.filename) search.set("filename", params.filename);
    return withMediaToken(`${baseURL}/api/v1/assets/${id}/export?${search.toString()}`);
  },

  getThumbnailUrl(id: string, size: "small" | "medium" | "large" = "small"): string {
    return withMediaToken(`${baseURL}/api/v1/assets/${id}/thumbnail?size=${size}`);
  },

  getWebVideoUrl(id: string): string {
    return withMediaToken(`${baseURL}/api/v1/assets/${id}/video/web`);
  },

  getWebAudioUrl(id: string): string {
    return withMediaToken(`${baseURL}/api/v1/assets/${id}/audio/web`);
  },

  getPersonCoverUrl(id: number | string, repositoryId?: string): string {
    const search = new URLSearchParams();
    if (repositoryId) {
      search.set("repository_id", repositoryId);
    }

    const suffix = search.toString();
    const url = `${baseURL}/api/v1/people/${id}/cover${suffix ? `?${suffix}` : ""}`;
    return withMediaToken(url);
  },

  getFaceCropUrl(
    personId: number | string,
    faceId: number | string,
    repositoryId?: string,
  ): string {
    const search = new URLSearchParams();
    if (repositoryId) {
      search.set("repository_id", repositoryId);
    }

    const suffix = search.toString();
    const url = `${baseURL}/api/v1/people/${personId}/faces/${faceId}/crop${suffix ? `?${suffix}` : ""}`;
    return withMediaToken(url);
  },
};
