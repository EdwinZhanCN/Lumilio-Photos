import React, { createContext, useReducer, useEffect, ReactNode } from "react";
import { authReducer, initialState } from "./auth.reducer";
import { AuthAction, AuthState } from "./types";
import { authService } from "@/services/authService";
import { getToken, removeToken, saveToken } from "@/lib/http-commons/api";

interface AuthContextType extends AuthState {
  dispatch: React.Dispatch<AuthAction>;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, email: string, password: string) => Promise<void>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(authReducer, initialState);

  useEffect(() => {
    const initAuth = async () => {
      const token = getToken();
      if (token) {
        dispatch({ type: "AUTH_START" });
        try {
          const response = await authService.getCurrentUser();
          // Use code 0 from schema/backend
          if (response.data.code === 0 && response.data.data) {
            dispatch({ type: "AUTH_SUCCESS", payload: response.data.data });
          } else {
            removeToken();
            dispatch({ type: "AUTH_FAILURE", payload: "Failed to get user" });
          }
        } catch (error) {
          removeToken();
          dispatch({ type: "AUTH_FAILURE", payload: "Session expired" });
        }
      }
    };

    initAuth();
  }, []);

  const login = async (username: string, password: string) => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await authService.login({ username, password });
      
      // Backend returns code 0 for success as per schema.d.ts and response.go
      if (response.data.code === 0 && response.data.data) {
        const { token, refreshToken, user } = response.data.data;
        if (token && user) {
          saveToken(token, refreshToken || "");
          dispatch({ type: "AUTH_SUCCESS", payload: user });
        } else {
          dispatch({ type: "AUTH_FAILURE", payload: "No token received" });
        }
      } else {
        dispatch({ type: "AUTH_FAILURE", payload: response.data.message || "Login failed" });
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
      if (response.data.code === 0 && response.data.data) {
        const { token, refreshToken, user } = response.data.data;
        if (token && user) {
          saveToken(token, refreshToken || "");
          dispatch({ type: "AUTH_SUCCESS", payload: user });
        } else {
          dispatch({ type: "AUTH_FAILURE", payload: "No token received" });
        }
      } else {
        dispatch({ type: "AUTH_FAILURE", payload: response.data.message || "Registration failed" });
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
