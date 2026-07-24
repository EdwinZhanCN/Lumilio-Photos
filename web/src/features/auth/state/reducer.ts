import { AuthAction, AuthState } from "../types.ts";
export const initialState: AuthState = {
  user: null,
  isAuthenticated: false,
  // The refresh credential is HttpOnly and intentionally invisible here, so
  // every fresh document performs one cookie-session probe before routing.
  isLoading: true,
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
    case "AUTH_IDLE":
      return {
        ...state,
        isLoading: false,
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
