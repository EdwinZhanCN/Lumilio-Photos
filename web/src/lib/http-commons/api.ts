/**
 * Token management utilities for authentication
 * 
 * This module provides functions for managing JWT tokens in localStorage.
 * Used by the openapi-fetch client for authentication.
 */

// JWT Token management
const TOKEN_KEY = "auth_token";
const REFRESH_TOKEN_KEY = "refresh_token";

/**
 * Get the current access token from localStorage
 */
export const getToken = () => localStorage.getItem(TOKEN_KEY);

/**
 * Get the current refresh token from localStorage
 */
export const getRefreshToken = () => localStorage.getItem(REFRESH_TOKEN_KEY);

/**
 * Save both access and refresh tokens to localStorage
 */
export const saveToken = (token: string, refreshToken: string) => {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  if (refreshToken) localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
};

/**
 * Remove both tokens from localStorage (logout)
 */
export const removeToken = () => {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
};
