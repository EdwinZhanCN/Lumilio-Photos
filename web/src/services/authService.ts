// src/services/authService.ts

import client from "@/lib/http-commons/client";
import { saveToken, removeToken } from "@/lib/http-commons/api";
import type { components } from "@/lib/http-commons/schema.d.ts";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];

export type User = Schemas["dto.UserDTO"];
export type AuthResponse = Schemas["dto.AuthResponseDTO"];
export type LoginRequest = Schemas["dto.LoginRequestDTO"];
export type RegisterRequest = Schemas["dto.RegisterRequestDTO"];
export type RefreshTokenRequest = Schemas["dto.RefreshTokenRequestDTO"];

// Helper type for API result
interface ApiResult<T = unknown> {
  code?: number;
  message?: string;
  data?: T;
}

// ============================================================================
// Auth Service
// ============================================================================

export const authService = {
  /**
   * Authenticates a user with username and password.
   */
  async login(request: LoginRequest) {
    const result = await client.POST("/auth/login", {
      body: request,
    });

    // Access the response data (openapi-fetch returns { data, error, response })
    const responseData = result.data as ApiResult<AuthResponse> | undefined;

    // Save tokens on successful login
    if (responseData?.code === 0 && responseData?.data) {
      const { token, refreshToken } = responseData.data;
      if (token && refreshToken) {
        saveToken(token, refreshToken);
      }
    }

    return result;
  },

  /**
   * Registers a new user account.
   */
  async register(request: RegisterRequest) {
    const result = await client.POST("/auth/register", {
      body: request,
    });

    const responseData = result.data as ApiResult<AuthResponse> | undefined;

    // Save tokens on successful registration
    if (responseData?.code === 0 && responseData?.data) {
      const { token, refreshToken } = responseData.data;
      if (token && refreshToken) {
        saveToken(token, refreshToken);
      }
    }

    return result;
  },

  /**
   * Refreshes the access token using a valid refresh token.
   */
  async refreshToken(request: RefreshTokenRequest) {
    return client.POST("/auth/refresh", {
      body: request,
    });
  },

  /**
   * Logs out the user by revoking the refresh token.
   */
  async logout(request: RefreshTokenRequest) {
    const result = await client.POST("/auth/logout", {
      body: request,
    });

    // Clear tokens on logout
    removeToken();

    return result;
  },

  /**
   * Gets information about the currently authenticated user.
   */
  async getCurrentUser() {
    return client.GET("/auth/me", {});
  },
};
