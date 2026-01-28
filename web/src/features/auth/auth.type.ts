import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type User = Schemas["dto.UserDTO"];
export type AuthResponse = Schemas["dto.AuthResponseDTO"];
export type LoginRequest = Schemas["dto.LoginRequestDTO"];
export type RegisterRequest = Schemas["dto.RegisterRequestDTO"];
export type RefreshTokenRequest = Schemas["dto.RefreshTokenRequestDTO"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
}

export type AuthAction =
  | { type: "AUTH_START" }
  | { type: "AUTH_SUCCESS"; payload: User }
  | { type: "AUTH_FAILURE"; payload: string }
  | { type: "LOGOUT" }
  | { type: "SET_USER"; payload: User | null };
