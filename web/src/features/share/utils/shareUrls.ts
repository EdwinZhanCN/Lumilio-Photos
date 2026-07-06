const baseURL = import.meta.env.VITE_API_URL ?? "";

/**
 * Public share media URL helpers. Unlike {@link assetUrls}, these never carry
 * a media token — the share token in the path is the only capability, and it
 * must never be reused as (or confused with) a normal authenticated media
 * token.
 */
export const shareUrls = {
  publicShareUrl(token: string): string {
    return `${window.location.origin}/s/${token}`;
  },

  getThumbnailUrl(
    token: string,
    assetId: string,
    size: "small" | "medium" | "large" = "small",
  ): string {
    return `${baseURL}/api/v1/public/shares/${token}/assets/${assetId}/thumbnail?size=${size}`;
  },

  getWebVideoUrl(token: string, assetId: string): string {
    return `${baseURL}/api/v1/public/shares/${token}/assets/${assetId}/web-video`;
  },

  getWebAudioUrl(token: string, assetId: string): string {
    return `${baseURL}/api/v1/public/shares/${token}/assets/${assetId}/web-audio`;
  },

  getOriginalUrl(token: string, assetId: string): string {
    return `${baseURL}/api/v1/public/shares/${token}/assets/${assetId}/original`;
  },

  getDownloadUrl(token: string): string {
    return `${baseURL}/api/v1/public/shares/${token}/download`;
  },
};
