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

  getThumbnailUrl(
    id: string,
    size: "small" | "medium" | "large" = "small",
  ): string {
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
};
