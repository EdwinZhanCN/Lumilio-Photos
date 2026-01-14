// src/services/authService.ts

import api from "@/lib/http-commons/api.ts";
import type { AxiosRequestConfig, AxiosResponse } from "axios";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { ApiResult } from "./uploadService";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];

/**
 * User response object
 */
export type User = Schemas["dto.UserDTO"];

/**
 * Authentication response (login/register/refresh)
 */
export type AuthResponse = Schemas["dto.AuthResponseDTO"];

/**
 * Login request
 */
export type LoginRequest = Schemas["dto.LoginRequestDTO"];

/**
 * Register request
 */
export type RegisterRequest = Schemas["dto.RegisterRequestDTO"];

/**
 * Refresh token request
 */
export type RefreshTokenRequest = Schemas["dto.RefreshTokenRequestDTO"];

// ============================================================================
// Auth Service
// ============================================================================

/**
 * @service AuthService
 * @description A collection of functions for authentication-related API endpoints.
 */
export const authService = {
  /**
   * Authenticates a user with username and password.
   * @param {LoginRequest} request - The login credentials (username, password).
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<AuthResponse>>>} A promise resolving to authentication tokens and user data.
   */
  login: async (
    request: LoginRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AuthResponse>>> => {
    return api.post<ApiResult<AuthResponse>>(
      "/api/v1/auth/login",
      request,
      config,
    );
  },

  /**
   * Registers a new user account.
   * @param {RegisterRequest} request - The registration data (username, email, password).
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<AuthResponse>>>} A promise resolving to authentication tokens and user data.
   */
  register: async (
    request: RegisterRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AuthResponse>>> => {
    return api.post<ApiResult<AuthResponse>>(
      "/api/v1/auth/register",
      request,
      config,
    );
  },

  /**
   * Refreshes the access token using a valid refresh token.
   * @param {RefreshTokenRequest} request - The refresh token.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<AuthResponse>>>} A promise resolving to new authentication tokens.
   */
  refreshToken: async (
    request: RefreshTokenRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AuthResponse>>> => {
    return api.post<ApiResult<AuthResponse>>(
      "/api/v1/auth/refresh",
      request,
      config,
    );
  },

  /**
   * Logs out the user by revoking the refresh token.
   * @param {RefreshTokenRequest} request - The refresh token to revoke.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<any>>>} A promise resolving to a success message.
   */
  logout: async (
    request: RefreshTokenRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<any>>> => {
    return api.post<ApiResult<any>>("/api/v1/auth/logout", request, config);
  },

  /**
   * Gets information about the currently authenticated user.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<User>>>} A promise resolving to the current user's information.
   */
  getCurrentUser: async (
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<User>>> => {
    return api.get<ApiResult<User>>("/api/v1/auth/me", config);
  },
};
