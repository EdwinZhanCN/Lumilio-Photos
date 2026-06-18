import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type {
  MFAStatus,
  RecoveryCodesResponse,
  TOTPSetupResponse,
} from "../auth.type.ts";

const mfaStatusQueryKey = ["get", "/api/v1/auth/mfa"];
const passkeysQueryKey = ["get", "/api/v1/auth/mfa/passkeys"];

export function useMFAStatus(): UseQueryResult<MFAStatus, unknown> {
  return $api.useQuery(
    "get",
    "/api/v1/auth/mfa",
    {},
    {
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<MFAStatus, unknown>;
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
      // Disabling TOTP cascades to passkeys on the server, so refresh both.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: mfaStatusQueryKey }),
        queryClient.invalidateQueries({ queryKey: passkeysQueryKey }),
      ]);
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

export type { MFAStatus, RecoveryCodesResponse, TOTPSetupResponse };
