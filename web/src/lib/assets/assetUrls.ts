const baseURL = import.meta.env.VITE_API_URL || "http://localhost:8080";

/**
 * Asset media URL helpers for use outside the data layer.
 */
export const assetUrls = {
  getOriginalFileUrl(id: string): string {
    return `${baseURL}/api/v1/assets/${id}/original`;
  },

  getThumbnailUrl(id: string, size: "small" | "medium" | "large" = "small"): string {
    return `${baseURL}/api/v1/assets/${id}/thumbnail?size=${size}`;
  },

  getWebVideoUrl(id: string): string {
    return `${baseURL}/api/v1/assets/${id}/video/web`;
  },

  getWebAudioUrl(id: string): string {
    return `${baseURL}/api/v1/assets/${id}/audio/web`;
  },
};
