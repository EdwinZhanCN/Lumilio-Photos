import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type CloudProviderStatus = Schemas["dto.CloudProviderStatusDTO"];

export function useCloudProviders(options?: { refetchInterval?: number }) {
  return $api.useQuery("get", "/api/v1/cloud/providers", {}, options) as UseQueryResult<
    ApiResult<{ providers: CloudProviderStatus[] }>,
    unknown
  >;
}

export function useConnectICloud() {
  return $api.useMutation("post", "/api/v1/cloud/icloud/connect");
}

export function useVerifyICloud2FA() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/icloud/verify-2fa", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/providers"] });
    },
  });
}

export function useTriggerSync() {
  return $api.useMutation("post", "/api/v1/cloud/sync");
}

export function useDisconnectCloud() {
  const queryClient = useQueryClient();
  return $api.useMutation("delete", "/api/v1/cloud/{provider}", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/providers"] });
    },
  });
}

export function useCreateRepository() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/repositories", {
    onSuccess: () => {
      // Invalidate both repositories list and working repos
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] });
    },
  });
}

export function useDeleteRepository() {
  const queryClient = useQueryClient();
  return $api.useMutation("delete", "/api/v1/repositories/{id}", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] });
    },
  });
}
