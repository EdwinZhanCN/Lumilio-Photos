import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type {
  ApiResult,
  PasskeyCredentialSummary,
  PasskeyListResponse,
  PasskeyOptionsResponse,
} from "../auth.type.ts";

const passkeysQueryKey = ["get", "/api/v1/auth/mfa/passkeys"];
const mfaStatusQueryKey = ["get", "/api/v1/auth/mfa"];

export function usePasskeys(): UseQueryResult<ApiResult<PasskeyListResponse>, unknown> & {
  passkeys: PasskeyCredentialSummary[];
  total: number;
} {
  const query = $api.useQuery(
    "get",
    "/api/v1/auth/mfa/passkeys",
    {},
    {
      refetchOnWindowFocus: false,
    },
  ) as UseQueryResult<ApiResult<PasskeyListResponse>, unknown>;

  return {
    ...query,
    passkeys: query.data?.data?.credentials ?? [],
    total: query.data?.data?.total ?? 0,
  };
}

export function useBeginPasskeyEnrollment() {
  return $api.useMutation("post", "/api/v1/auth/mfa/passkeys/options");
}

export function useVerifyPasskeyEnrollment() {
  const queryClient = useQueryClient();

  return $api.useMutation("post", "/api/v1/auth/mfa/passkeys/verify", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: passkeysQueryKey }),
        queryClient.invalidateQueries({ queryKey: mfaStatusQueryKey }),
      ]);
    },
  });
}

export function useDeletePasskey() {
  const queryClient = useQueryClient();

  return $api.useMutation("delete", "/api/v1/auth/mfa/passkeys/{id}", {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: passkeysQueryKey }),
        queryClient.invalidateQueries({ queryKey: mfaStatusQueryKey }),
      ]);
    },
  });
}

export type { PasskeyOptionsResponse };
