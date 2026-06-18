import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
export type CloudCredential = Schemas["dto.CloudCredentialDTO"];
export type CloudProvider = Schemas["dto.CloudProviderDTO"];
export type CloudAuthChallenge = Schemas["dto.CloudAuthChallengeDTO"];
export type CloudProviderField = Schemas["dto.CloudProviderFieldDTO"];
export type CloudImportRun = Schemas["dto.CloudImportRunDTO"];
export type RepositoryCloudStatus = Schemas["dto.RepositoryCloudStatusDTO"];

export function useCloudProviders() {
  return $api.useQuery("get", "/api/v1/cloud/providers", {}) as UseQueryResult<
    { providers: CloudProvider[] },
    unknown
  >;
}

export function useCloudCredentials(options?: { refetchInterval?: number }) {
  return $api.useQuery("get", "/api/v1/cloud/credentials", {}, options) as UseQueryResult<
    { credentials: CloudCredential[] },
    unknown
  >;
}

export function useCreateCloudCredential() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/credentials", {
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/credentials"] });
    },
  });
}

export function useVerifyCloudCredentialChallenge() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/credentials/{id}/auth-challenge", {
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/credentials"] });
    },
  });
}

export function useDisconnectCloudCredential() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/credentials/{id}/disconnect", {
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/credentials"] });
    },
  });
}

export function useReconnectCloudCredential() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/cloud/credentials/{id}/reconnect", {
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/credentials"] });
    },
  });
}

export function useRemoveCloudCredential() {
  const queryClient = useQueryClient();
  return $api.useMutation("delete", "/api/v1/cloud/credentials/{id}", {
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/cloud/credentials"] });
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/indexing/repositories"] });
    },
  });
}

export function useRepositoryCloudStatus(repositoryId: string, enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/repositories/{id}/cloud",
    {
      params: {
        path: {
          id: repositoryId,
        },
      },
    },
    {
      enabled: enabled && Boolean(repositoryId),
      // Only poll while an import is actively in progress; otherwise fetch once.
      // Avoids every repository card hammering this endpoint every 5s forever.
      refetchInterval: (query) => {
        const status = (query.state.data as RepositoryCloudStatus | undefined)
          ?.latest_run?.status;
        return status === "running" || status === "queued" ? 5000 : false;
      },
      staleTime: 2000,
    },
  ) as UseQueryResult<RepositoryCloudStatus, unknown>;
}

export function useStartRepositoryCloudImport() {
  const queryClient = useQueryClient();
  return $api.useMutation("post", "/api/v1/repositories/{id}/cloud/import", {
    onSuccess: (_data, variables) => {
      const id = variables?.params?.path?.id;
      if (id) {
        void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories/{id}/cloud"] });
      }
      void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/indexing/repositories"] });
    },
  });
}
