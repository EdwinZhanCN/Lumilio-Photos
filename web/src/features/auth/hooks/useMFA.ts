import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type {
  ApiResult,
  MFAStatus,
  RecoveryCodesResponse,
  TOTPSetupResponse,
} from "../auth.type.ts";

const mfaStatusQueryKey = ["get", "/api/v1/auth/mfa"];

export function useMFAStatus(): UseQueryResult<ApiResult<MFAStatus>, unknown> {
  return $api.useQuery(
    "get",
    "/api/v1/auth/mfa",
    {},
    {
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<MFAStatus>, unknown>;
}

export function useBeginTOTPSetup() {
  return $api.useMutation("post", "/api/v1/auth/mfa/totp/setup");
}

export function useEnableTOTP() {
  const queryClient = useQueryClient();

  return $api.useMutation("post", "/api/v1/auth/mfa/totp/enable", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: mfaStatusQueryKey });
    },
  });
}

export function useDisableTOTP() {
  const queryClient = useQueryClient();

  return $api.useMutation("post", "/api/v1/auth/mfa/totp/disable", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: mfaStatusQueryKey });
    },
  });
}

export function useRegenerateRecoveryCodes() {
  const queryClient = useQueryClient();

  return $api.useMutation("post", "/api/v1/auth/mfa/recovery-codes/regenerate", {
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: mfaStatusQueryKey });
    },
  });
}

export type {
  ApiResult,
  MFAStatus,
  RecoveryCodesResponse,
  TOTPSetupResponse,
};
