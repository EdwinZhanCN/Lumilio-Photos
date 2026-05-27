import { client } from "@/lib/http-commons/client";
import {
  getMediaToken,
  getMediaTokenExpiresAt,
  getToken,
  removeMediaToken,
  saveMediaToken,
} from "@/lib/http-commons/auth.ts";

const MEDIA_TOKEN_REFRESH_BUFFER_MS = 60_000;
const MEDIA_TOKEN_REFRESH_INTERVAL_MS = 60_000;

let mediaTokenRequestInFlight: Promise<string | null> | null = null;

const shouldRefreshMediaToken = (expiresAt: number | null) => {
  if (!expiresAt) {
    return true;
  }
  return expiresAt - Date.now() <= MEDIA_TOKEN_REFRESH_BUFFER_MS;
};

const requestMediaToken = async (): Promise<string | null> => {
  const { data, error, response } = await client.GET("/api/v1/auth/media-token");

  if (response?.status === 401 || response?.status === 403) {
    removeMediaToken();
    return null;
  }
  if (error || !data) {
    throw new Error(`media token request failed: ${JSON.stringify(error)}`);
  }

  const token = data.data?.token;
  const expiresAt = data.data?.expires_at;
  if (!token || !expiresAt) {
    throw new Error("media token response is missing required fields");
  }

  saveMediaToken(token, expiresAt);
  return token;
};

export const getMediaTokenRefreshIntervalMs = () =>
  MEDIA_TOKEN_REFRESH_INTERVAL_MS;

export const ensureMediaToken = async (
  force = false,
): Promise<string | null> => {
  const accessToken = getToken();
  if (!accessToken) {
    removeMediaToken();
    return null;
  }

  const existingToken = getMediaToken();
  const expiresAt = getMediaTokenExpiresAt();

  if (!force && existingToken && !shouldRefreshMediaToken(expiresAt)) {
    return existingToken;
  }

  if (mediaTokenRequestInFlight) {
    return mediaTokenRequestInFlight;
  }

  mediaTokenRequestInFlight = requestMediaToken()
    .catch((error: unknown) => {
      const isExistingTokenStillValid =
        !!existingToken && !!expiresAt && expiresAt > Date.now();
      if (isExistingTokenStillValid) {
        console.warn(
          "Failed to refresh media token, keeping current token:",
          error,
        );
        return existingToken;
      }
      console.warn("Failed to fetch media token:", error);
      return null;
    })
    .finally(() => {
      mediaTokenRequestInFlight = null;
    });

  return mediaTokenRequestInFlight;
};
