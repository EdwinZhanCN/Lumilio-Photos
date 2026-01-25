import React, { createContext, useReducer, useEffect, ReactNode, useRef } from "react";
import { authReducer, initialState } from "./auth.reducer";
import { AuthAction, AuthState } from "./auth.types.ts";
import { authService, User } from "@/services/authService";
import { getToken, getRefreshToken, removeToken, saveToken } from "@/lib/http-commons/api";

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
          const response = await authService.getCurrentUser();
          if (response.data?.code === 0 && response.data?.data) {
            dispatch({ type: "AUTH_SUCCESS", payload: response.data.data as User });
            isInitialized.current = true;
            return;
          }
        }

        // 2. If access token failed or missing, try refresh token
        if (refreshToken) {
          const refreshRes = await authService.refreshToken({ refreshToken });
          if (refreshRes.data?.code === 0 && refreshRes.data?.data) {
            const { token: newToken, refreshToken: newRefreshToken, user } = refreshRes.data.data;
            saveToken(newToken!, newRefreshToken || refreshToken);
            dispatch({ type: "AUTH_SUCCESS", payload: user! });
            isInitialized.current = true;
            return;
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
      const response = await authService.login({ username, password });
      if (response.data?.code === 0 && response.data?.data) {
        const { token, refreshToken, user } = response.data.data;
        if (token && user) {
          saveToken(token, refreshToken || "");
          dispatch({ type: "AUTH_SUCCESS", payload: user });
        }
      } else {
        dispatch({ type: "AUTH_FAILURE", payload: response.data?.message || "Login failed" });
      }
    } catch (error: any) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: error.response?.data?.message || "Invalid credentials",
      });
      throw error;
    }
  };

  const register = async (username: string, email: string, password: string) => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await authService.register({ username, email, password });
      if (response.data?.code === 0 && response.data?.data) {
        const { token, refreshToken, user } = response.data.data;
        if (token && user) {
          saveToken(token, refreshToken || "");
          dispatch({ type: "AUTH_SUCCESS", payload: user });
        }
      } else {
        dispatch({ type: "AUTH_FAILURE", payload: response.data?.message || "Registration failed" });
      }
    } catch (error: any) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: error.response?.data?.message || "Registration failed",
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
