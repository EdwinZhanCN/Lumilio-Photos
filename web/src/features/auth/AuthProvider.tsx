import React, { createContext, useReducer, useEffect, ReactNode, useRef } from "react";
import { authReducer, initialState } from "./auth.reducer";
import {
  ApiResult,
  AuthAction,
  AuthResponse,
  AuthState,
  LoginResult,
  MFAMethod,
  User,
} from "./auth.type.ts";
import { getToken, getRefreshToken, removeToken, saveToken } from "@/lib/http-commons/auth.ts";
import { $api } from "@/lib/http-commons/queryClient";

interface AuthContextType extends AuthState {
  dispatch: React.Dispatch<AuthAction>;
  login: (username: string, password: string) => Promise<LoginResult>;
  verifyMFA: (mfaToken: string, code: string, method: MFAMethod) => Promise<void>;
  completeAuth: (response: AuthResponse) => User;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(authReducer, initialState);
  const isInitialized = useRef(false);
  const currentUserMutation = $api.useMutation("get", "/api/v1/auth/me");
  const refreshMutation = $api.useMutation("post", "/api/v1/auth/refresh");
  const loginMutation = $api.useMutation("post", "/api/v1/auth/login");
  const verifyMFAMutation = $api.useMutation("post", "/api/v1/auth/mfa/verify");

  const getApiMessage = (error: unknown, fallback: string) => {
    if (!error || typeof error !== "object") return fallback;
    const apiError = error as ApiResult;
    return apiError.message || apiError.error || fallback;
  };

  const completeAuth = (response: AuthResponse): User => {
    const { token, refreshToken, user } = response;
    if (!token || !user) {
      throw new Error("Authentication response did not include a session.");
    }

    saveToken(token, refreshToken || "");
    dispatch({ type: "AUTH_SUCCESS", payload: user });
    return user;
  };

  useEffect(() => {
    if (isInitialized.current) return;

    const initAuth = async () => {
      const token = getToken();
      const refreshToken = getRefreshToken();

      if (!token && !refreshToken) {
        dispatch({ type: "AUTH_FAILURE", payload: null as any });
        isInitialized.current = true;
        return;
      }

      try {
        // 1. Try to get current user with existing access token
        if (token) {
          try {
            const response = await currentUserMutation.mutateAsync({});
            const responseData = response as ApiResult<User> | undefined;
            if (responseData?.code === 0 && responseData?.data) {
              dispatch({ type: "AUTH_SUCCESS", payload: responseData.data });
              isInitialized.current = true;
              return;
            }
          } catch (error) {
            console.warn("Auth token validation failed:", error);
          }
        }

        // 2. If access token failed or missing, try refresh token
        if (refreshToken) {
          try {
            const refreshRes = await refreshMutation.mutateAsync({
              body: { refreshToken },
            });
            const refreshData = refreshRes as ApiResult<AuthResponse> | undefined;
            if (refreshData?.code === 0 && refreshData?.data) {
              completeAuth(refreshData.data);
              isInitialized.current = true;
              return;
            }
          } catch (error) {
            console.warn("Token refresh failed:", error);
          }
        }

        // 3. Everything failed
        removeToken();
        dispatch({ type: "AUTH_FAILURE", payload: "Session expired" });
      } catch (error) {
        console.error("Auth initialization failed:", error);
        removeToken();
        dispatch({ type: "AUTH_FAILURE", payload: "Authentication failed" });
      } finally {
        isInitialized.current = true;
      }
    };

    initAuth();
  }, []);

  const login = async (username: string, password: string): Promise<LoginResult> => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await loginMutation.mutateAsync({
        body: { username, password },
      });
      const responseData = response as ApiResult<AuthResponse> | undefined;
      if (responseData?.code === 0 && responseData?.data) {
        const { requires_mfa, mfa_token, mfa_methods, user } = responseData.data;
        if (requires_mfa && mfa_token) {
          dispatch({ type: "AUTH_IDLE" });
          return {
            status: "mfa_required",
            challenge: {
              user: user ?? null,
              mfaToken: mfa_token,
              mfaMethods: (mfa_methods ?? []) as MFAMethod[],
            },
          };
        }
        if (user) {
          completeAuth(responseData.data);
          return { status: "authenticated" };
        }
      } else {
        dispatch({
          type: "AUTH_FAILURE",
          payload: responseData?.message || "Login failed",
        });
      }

      throw new Error(responseData?.message || "Login failed");
    } catch (error: any) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: getApiMessage(error, "Invalid credentials"),
      });
      throw error;
    }
  };

  const verifyMFA = async (mfaToken: string, code: string, method: MFAMethod) => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await verifyMFAMutation.mutateAsync({
        body: { mfa_token: mfaToken, code, method },
      });
      const responseData = response as ApiResult<AuthResponse> | undefined;
      if (responseData?.code === 0 && responseData?.data) {
        if (responseData.data.user) {
          completeAuth(responseData.data);
          return;
        }
      }

      const message = responseData?.message || "Verification failed";
      dispatch({ type: "AUTH_FAILURE", payload: message });
      throw new Error(message);
    } catch (error: any) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: getApiMessage(error, "Verification failed"),
      });
      throw error;
    }
  };

  const logout = () => {
    removeToken();
    dispatch({ type: "LOGOUT" });
  };

  return (
    <AuthContext.Provider
      value={{ ...state, dispatch, login, verifyMFA, completeAuth, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
};
