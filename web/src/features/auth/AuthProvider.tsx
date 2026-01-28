import React, { createContext, useReducer, useEffect, ReactNode, useRef } from "react";
import { authReducer, initialState } from "./auth.reducer";
import { ApiResult, AuthAction, AuthResponse, AuthState, User } from "./auth.type.ts";
import { getToken, getRefreshToken, removeToken, saveToken } from "@/lib/http-commons/auth.ts";
import { $api } from "@/lib/http-commons/queryClient";

interface AuthContextType extends AuthState {
  dispatch: React.Dispatch<AuthAction>;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, email: string, password: string) => Promise<void>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(authReducer, initialState);
  const isInitialized = useRef(false);
  const currentUserMutation = $api.useMutation("get", "/api/v1/auth/me");
  const refreshMutation = $api.useMutation("post", "/api/v1/auth/refresh");
  const loginMutation = $api.useMutation("post", "/api/v1/auth/login");
  const registerMutation = $api.useMutation("post", "/api/v1/auth/register");

  const getApiMessage = (error: unknown, fallback: string) => {
    if (!error || typeof error !== "object") return fallback;
    const apiError = error as ApiResult;
    return apiError.message || apiError.error || fallback;
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
              const { token: newToken, refreshToken: newRefreshToken, user } = refreshData.data;
              if (newToken && user) {
                saveToken(newToken, newRefreshToken || refreshToken);
                dispatch({ type: "AUTH_SUCCESS", payload: user });
                isInitialized.current = true;
                return;
              }
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

  const login = async (username: string, password: string) => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await loginMutation.mutateAsync({
        body: { username, password },
      });
      const responseData = response as ApiResult<AuthResponse> | undefined;
      if (responseData?.code === 0 && responseData?.data) {
        const { token, refreshToken, user } = responseData.data;
        if (token && user) {
          saveToken(token, refreshToken || "");
          dispatch({ type: "AUTH_SUCCESS", payload: user });
        }
      } else {
        dispatch({
          type: "AUTH_FAILURE",
          payload: responseData?.message || "Login failed",
        });
      }
    } catch (error: any) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: getApiMessage(error, "Invalid credentials"),
      });
      throw error;
    }
  };

  const register = async (username: string, email: string, password: string) => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await registerMutation.mutateAsync({
        body: { username, email, password },
      });
      const responseData = response as ApiResult<AuthResponse> | undefined;
      if (responseData?.code === 0 && responseData?.data) {
        const { token, refreshToken, user } = responseData.data;
        if (token && user) {
          saveToken(token, refreshToken || "");
          dispatch({ type: "AUTH_SUCCESS", payload: user });
        }
      } else {
        dispatch({
          type: "AUTH_FAILURE",
          payload: responseData?.message || "Registration failed",
        });
      }
    } catch (error: any) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: getApiMessage(error, "Registration failed"),
      });
      throw error;
    }
  };

  const logout = () => {
    removeToken();
    dispatch({ type: "LOGOUT" });
  };

  return (
    <AuthContext.Provider value={{ ...state, dispatch, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
};
