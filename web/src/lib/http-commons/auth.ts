/**
 * Token management utilities for authentication
 *
 * The short-lived access token and session-bound CSRF proof are
 * browser-readable. The CSRF proof is not an independent credential, but still
 * must not leak cross-origin. The long-lived refresh credential is an HttpOnly
 * cookie and never enters this module.
 */

import { resetSessionExpiredNotification } from "./sessionEvents.ts";

// JWT Token management
const TOKEN_KEY = "auth_token";
const CSRF_TOKEN_KEY = "csrf_token";
const MEDIA_TOKEN_KEY = "media_token";
const MEDIA_TOKEN_EXPIRES_AT_KEY = "media_token_expires_at";

const hasStorage = () => typeof localStorage !== "undefined";

/**
 * Get the current access token from localStorage
 */
export const getToken = () => (hasStorage() ? localStorage.getItem(TOKEN_KEY) : null);

/**
 * Get the CSRF proof bound to the current HttpOnly refresh-cookie session.
 */
export const getCSRFToken = () => (hasStorage() ? localStorage.getItem(CSRF_TOKEN_KEY) : null);

/**
 * Save the short-lived access token and session-bound CSRF proof.
 */
export const saveToken = (token: string, csrfToken: string) => {
  if (!hasStorage()) return;
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
  if (csrfToken) localStorage.setItem(CSRF_TOKEN_KEY, csrfToken);
  else localStorage.removeItem(CSRF_TOKEN_KEY);
  resetSessionExpiredNotification();
};

export const saveCSRFToken = (csrfToken: string) => {
  if (!hasStorage()) return;
  if (csrfToken) localStorage.setItem(CSRF_TOKEN_KEY, csrfToken);
  else localStorage.removeItem(CSRF_TOKEN_KEY);
};

export const getMediaToken = () => (hasStorage() ? localStorage.getItem(MEDIA_TOKEN_KEY) : null);

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
 * Remove all browser-readable session material (logout)
 */
export const removeToken = () => {
  if (!hasStorage()) return;
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(CSRF_TOKEN_KEY);
  localStorage.removeItem(MEDIA_TOKEN_KEY);
  localStorage.removeItem(MEDIA_TOKEN_EXPIRES_AT_KEY);
};
