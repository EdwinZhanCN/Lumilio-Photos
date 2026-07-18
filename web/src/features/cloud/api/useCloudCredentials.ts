import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";

export function useCloudProviders() {
  return $api.useQuery("get", "/api/v1/cloud/providers", {});
}

export function useCloudCredentials(options?: { refetchInterval?: number }) {
  return $api.useQuery("get", "/api/v1/cloud/credentials", {}, options);
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
      void queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/indexing/repositories"],
      });
    },
  });
}
