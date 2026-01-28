import { AuthAction, AuthState } from "./auth.type.ts";
import { getToken, getRefreshToken } from "@/lib/http-commons/auth.ts";

// Production approach: Start in loading state if we have tokens to verify
const hasTokens = !!(getToken() || getRefreshToken());

export const initialState: AuthState = {
  user: null,
  isAuthenticated: false,
  isLoading: hasTokens, 
  error: null,
};

export function authReducer(state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case "AUTH_START":
      return {
        ...state,
        isLoading: true,
        error: null,
      };
    case "AUTH_SUCCESS":
      return {
        ...state,
        isLoading: false,
        isAuthenticated: true,
        user: action.payload,
        error: null,
      };
    case "AUTH_FAILURE":
      return {
        ...state,
        isLoading: false,
        isAuthenticated: false,
        user: null,
        error: action.payload,
      };
    case "LOGOUT":
      return {
        ...initialState,
        isLoading: false,
        isAuthenticated: false,
      };
    case "SET_USER":
      return {
        ...state,
        user: action.payload,
        isAuthenticated: !!action.payload,
        isLoading: false,
      };
    default:
      return state;
  }
}
