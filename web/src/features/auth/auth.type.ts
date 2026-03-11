import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type User = Schemas["dto.UserDTO"];
export type AuthResponse = Schemas["dto.AuthResponseDTO"];
export type LoginRequest = Schemas["dto.LoginRequestDTO"];
export type RefreshTokenRequest = Schemas["dto.RefreshTokenRequestDTO"];
export type MFAStatus = Schemas["dto.MFAStatusDTO"];
export type TOTPSetupResponse = Schemas["dto.TOTPSetupResponseDTO"];
export type RecoveryCodesResponse = Schemas["dto.RecoveryCodesResponseDTO"];
export type VerifyMFARequest = Schemas["dto.VerifyMFARequestDTO"];
export type BootstrapStatus = Schemas["dto.BootstrapStatusDTO"];
export type RegistrationStartRequest =
  Schemas["dto.RegistrationStartRequestDTO"];
export type RegistrationStartResponse =
  Schemas["dto.RegistrationStartResponseDTO"];
export type RegistrationSessionRequest =
  Schemas["dto.RegistrationSessionRequestDTO"];
export type RegistrationPasskeyVerifyRequest =
  Schemas["dto.RegistrationPasskeyVerifyRequestDTO"];
export type RegistrationTOTPSetupResponse =
  Schemas["dto.RegistrationTOTPSetupResponseDTO"];
export type RegistrationTOTPCompleteRequest =
  Schemas["dto.RegistrationTOTPCompleteRequestDTO"];
export type RegistrationTOTPCompleteResponse =
  Schemas["dto.RegistrationTOTPCompleteResponseDTO"];
export type PasskeyOptionsRequest = Schemas["dto.PasskeyOptionsRequestDTO"];
export type PasskeyOptionsResponse = Schemas["dto.PasskeyOptionsResponseDTO"];
export type PasskeyVerifyRequest = Schemas["dto.PasskeyVerifyRequestDTO"];
export type PasskeyCredentialSummary =
  Schemas["dto.PasskeyCredentialSummaryDTO"];
export type PasskeyListResponse = Schemas["dto.PasskeyListResponseDTO"];
export type ChangePasswordRequest = Schemas["dto.ChangePasswordRequestDTO"];
export type ResetAccessResponse = Schemas["dto.ResetAccessResponseDTO"];

export type MFAMethod = "totp" | "recovery_code";

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type LoginResult =
  | { status: "authenticated" }
  | {
      status: "mfa_required";
      challenge: {
        user: User | null;
        mfaToken: string;
        mfaMethods: MFAMethod[];
      };
    };

export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
}

export type AuthAction =
  | { type: "AUTH_START" }
  | { type: "AUTH_IDLE" }
  | { type: "AUTH_SUCCESS"; payload: User }
  | { type: "AUTH_FAILURE"; payload: string }
  | { type: "LOGOUT" }
  | { type: "SET_USER"; payload: User | null };
