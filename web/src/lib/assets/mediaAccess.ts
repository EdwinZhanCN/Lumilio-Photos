import { baseUrl } from "@/lib/http-commons/client";
import {
  getMediaToken,
  getMediaTokenExpiresAt,
  getToken,
  removeMediaToken,
  saveMediaToken,
} from "@/lib/http-commons/auth.ts";

type MediaTokenPayload = {
  token?: string;
  expires_at?: string;
};

type ApiResult<T> = {
  code?: number;
  data?: T;
};

const MEDIA_TOKEN_REFRESH_BUFFER_MS = 60_000;
const MEDIA_TOKEN_REFRESH_INTERVAL_MS = 60_000;

let mediaTokenRequestInFlight: Promise<string | null> | null = null;

const shouldRefreshMediaToken = (expiresAt: number | null) => {
  if (!expiresAt) {
    return true;
  }
  return expiresAt - Date.now() <= MEDIA_TOKEN_REFRESH_BUFFER_MS;
};

const requestMediaToken = async (accessToken: string): Promise<string | null> => {
  const response = await fetch(`${baseUrl}/api/v1/auth/media-token`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  if (response.status === 401 || response.status === 403) {
    removeMediaToken();
    return null;
  }
  if (!response.ok) {
    throw new Error(`media token request failed with status ${response.status}`);
  }

  const result = (await response.json()) as ApiResult<MediaTokenPayload>;
  const token = result?.data?.token;
  const expiresAt = result?.data?.expires_at;
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

  mediaTokenRequestInFlight = requestMediaToken(accessToken)
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
