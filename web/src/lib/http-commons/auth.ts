/**
 * Token management utilities for authentication
 * 
 * This module provides functions for managing JWT tokens in localStorage.
 * Used by the openapi-fetch client for authentication.
 */

// JWT Token management
const TOKEN_KEY = "auth_token";
const REFRESH_TOKEN_KEY = "refresh_token";
const MEDIA_TOKEN_KEY = "media_token";
const MEDIA_TOKEN_EXPIRES_AT_KEY = "media_token_expires_at";

const hasStorage = () => typeof localStorage !== "undefined";

/**
 * Get the current access token from localStorage
 */
export const getToken = () =>
  hasStorage() ? localStorage.getItem(TOKEN_KEY) : null;

/**
 * Get the current refresh token from localStorage
 */
export const getRefreshToken = () =>
  hasStorage() ? localStorage.getItem(REFRESH_TOKEN_KEY) : null;

/**
 * Save both access and refresh tokens to localStorage
 */
export const saveToken = (token: string, refreshToken: string) => {
  if (!hasStorage()) return;
  if (token) localStorage.setItem(TOKEN_KEY, token);
  if (refreshToken) localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
};

export const getMediaToken = () =>
  hasStorage() ? localStorage.getItem(MEDIA_TOKEN_KEY) : null;

export const getMediaTokenExpiresAt = (): number | null => {
  if (!hasStorage()) return null;
  const raw = localStorage.getItem(MEDIA_TOKEN_EXPIRES_AT_KEY);
  if (!raw) return null;
  const parsed = Number(raw);
  if (Number.isNaN(parsed) || parsed <= 0) return null;
  return parsed;
};

export const saveMediaToken = (token: string, expiresAtISO: string) => {
  if (!hasStorage()) return;
  if (!token) {
    localStorage.removeItem(MEDIA_TOKEN_KEY);
    localStorage.removeItem(MEDIA_TOKEN_EXPIRES_AT_KEY);
    return;
  }

  localStorage.setItem(MEDIA_TOKEN_KEY, token);
  const expiresAtMs = Date.parse(expiresAtISO);
  if (Number.isNaN(expiresAtMs)) {
    localStorage.removeItem(MEDIA_TOKEN_EXPIRES_AT_KEY);
    return;
  }
  localStorage.setItem(MEDIA_TOKEN_EXPIRES_AT_KEY, String(expiresAtMs));
};

export const removeMediaToken = () => {
  if (!hasStorage()) return;
  localStorage.removeItem(MEDIA_TOKEN_KEY);
  localStorage.removeItem(MEDIA_TOKEN_EXPIRES_AT_KEY);
};

/**
 * Remove both tokens from localStorage (logout)
 */
export const removeToken = () => {
  if (!hasStorage()) return;
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(MEDIA_TOKEN_KEY);
  localStorage.removeItem(MEDIA_TOKEN_EXPIRES_AT_KEY);
};
