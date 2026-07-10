import React, {
  createContext,
  useReducer,
  useEffect,
  ReactNode,
  useRef,
  useCallback,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import { authReducer, initialState } from "./auth.reducer";
import { AuthAction, AuthResponse, AuthState, LoginResult, MFAMethod, User } from "./auth.type.ts";
import { getToken, getRefreshToken, saveToken } from "@/lib/http-commons/auth.ts";
import { $api } from "@/lib/http-commons/queryClient";
import { ensureMediaToken, getMediaTokenRefreshIntervalMs } from "@/lib/assets/mediaAccess.ts";
import { useGlobal } from "@/contexts/GlobalContext.tsx";
import { resetSession } from "./resetSession.ts";
import { registerSessionExpiredHandler } from "./sessionEvents.ts";

interface AuthContextType extends AuthState {
  dispatch: React.Dispatch<AuthAction>;
  login: (username: string, password: string) => Promise<LoginResult>;
  verifyMFA: (mfaToken: string, code: string, method: MFAMethod) => Promise<void>;
  completeAuth: (response: AuthResponse) => Promise<User>;
  logout: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

const isMFAMethod = (value: string): value is MFAMethod =>
  value === "totp" || value === "recovery_code";

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(authReducer, initialState);
  const queryClient = useQueryClient();
  const { resetSessionState } = useGlobal();
  const isInitialized = useRef(false);
  const currentUserMutation = $api.useMutation("get", "/api/v1/auth/me");
  const refreshMutation = $api.useMutation("post", "/api/v1/auth/refresh");
  const loginMutation = $api.useMutation("post", "/api/v1/auth/login");
  const verifyMFAMutation = $api.useMutation("post", "/api/v1/auth/mfa/verify");
  const logoutMutation = $api.useMutation("post", "/api/v1/auth/logout");

  const resetClientSession = useCallback(
    () => resetSession({ queryClient, resetGlobalState: resetSessionState }),
    [queryClient, resetSessionState],
  );

  useEffect(
    () =>
      registerSessionExpiredHandler(async () => {
        await resetClientSession();
        dispatch({ type: "AUTH_FAILURE", payload: "auth.errors.sessionExpired" });
      }),
    [resetClientSession],
  );

  const getApiMessage = (error: unknown, fallback: string) => {
    if (!error || typeof error !== "object") return fallback;
    const apiError = error as { message?: string; error?: string };
    return apiError.message || apiError.error || fallback;
  };

  const completeAuth = async (response: AuthResponse): Promise<User> => {
    const { token, refreshToken, user } = response;
    if (!token || !refreshToken || !user) {
      throw new Error("auth.errors.invalidSessionResponse");
    }

    saveToken(token, refreshToken || "");
    await ensureMediaToken(true);
    dispatch({ type: "AUTH_SUCCESS", payload: user });
    return user;
  };

  useEffect(() => {
    if (isInitialized.current) return;

    const initAuth = async () => {
      const token = getToken();
      const refreshToken = getRefreshToken();

      if (!token && !refreshToken) {
        // No stored session: not authenticated, but this is idle, not a failure.
        dispatch({ type: "AUTH_IDLE" });
        isInitialized.current = true;
        return;
      }

      try {
        // 1. Try to get current user with existing access token
        if (token) {
          try {
            const response = await currentUserMutation.mutateAsync({});
            const responseData = response;
            if (responseData) {
              await ensureMediaToken();
              dispatch({ type: "AUTH_SUCCESS", payload: responseData });
              isInitialized.current = true;
              return;
            }
          } catch (error) {
            console.warn("Auth token validation failed:", error);
          }
        }

        // 2. If access token failed or missing, try refresh token
        if (!token && refreshToken) {
          try {
            const refreshRes = await refreshMutation.mutateAsync({
              body: { refreshToken },
            });
            const refreshData = refreshRes;
            if (refreshData) {
              await completeAuth(refreshData);
              isInitialized.current = true;
              return;
            }
          } catch (error) {
            console.warn("Token refresh failed:", error);
          }
        }

        // 3. Everything failed
        await resetClientSession();
        dispatch({
          type: "AUTH_FAILURE",
          payload: "auth.errors.sessionExpired",
        });
      } catch (error) {
        console.error("Auth initialization failed:", error);
        await resetClientSession();
        dispatch({
          type: "AUTH_FAILURE",
          payload: "auth.errors.authenticationFailed",
        });
      } finally {
        isInitialized.current = true;
      }
    };

    void initAuth();
  }, [resetClientSession]);

  useEffect(() => {
    if (!state.isAuthenticated) {
      return undefined;
    }

    void ensureMediaToken();
    const timer = window.setInterval(() => {
      void ensureMediaToken();
    }, getMediaTokenRefreshIntervalMs());

    return () => {
      window.clearInterval(timer);
    };
  }, [state.isAuthenticated]);

  const login = async (username: string, password: string): Promise<LoginResult> => {
    dispatch({ type: "AUTH_START" });
    try {
      const response = await loginMutation.mutateAsync({
        body: { username, password },
      });
      const responseData = response;
      if (responseData) {
        const { requires_mfa, mfa_token, mfa_methods, user } = responseData;
        if (requires_mfa && mfa_token) {
          dispatch({ type: "AUTH_IDLE" });
          return {
            status: "mfa_required",
            challenge: {
              user: user ?? null,
              mfaToken: mfa_token,
              mfaMethods: (mfa_methods ?? []).filter(isMFAMethod),
            },
          };
        }
        if (user) {
          await completeAuth(responseData);
          return { status: "authenticated" };
        }
      } else {
        dispatch({
          type: "AUTH_FAILURE",
          payload: "auth.errors.loginFailed",
        });
      }

      throw new Error("auth.errors.loginFailed");
    } catch (error: unknown) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: getApiMessage(error, "auth.errors.invalidCredentials"),
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
      const responseData = response;
      if (responseData) {
        if (responseData.user) {
          await completeAuth(responseData);
          return;
        }
      }

      const message = "auth.errors.verificationFailed";
      dispatch({ type: "AUTH_FAILURE", payload: message });
      throw new Error(message);
    } catch (error: unknown) {
      dispatch({
        type: "AUTH_FAILURE",
        payload: getApiMessage(error, "auth.errors.verificationFailed"),
      });
      throw error;
    }
  };

  const logout = async () => {
    // Best-effort server-side revocation of the current device's refresh token.
    // Always clear local state afterwards, even if the request fails, so the
    // user is never trapped in a logged-in UI.
    const refreshToken = getRefreshToken();
    if (refreshToken) {
      try {
        await logoutMutation.mutateAsync({ body: { refreshToken } });
      } catch (error) {
        console.warn("Logout request failed; clearing local session anyway:", error);
      }
    }
    await resetClientSession();
    dispatch({ type: "LOGOUT" });
  };

  return (
    <AuthContext.Provider value={{ ...state, dispatch, login, verifyMFA, completeAuth, logout }}>
      {children}
    </AuthContext.Provider>
  );
};
