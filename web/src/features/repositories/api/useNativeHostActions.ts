import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";

export type HostAction = components["schemas"]["dto.HostActionDTO"];
export type HostActionKind = components["schemas"]["dto.CreateHostActionRequestDTO"]["kind"];
export type HostActionResolution =
  components["schemas"]["dto.ResolveHostActionRequestDTO"]["resolution"];

export function useNativeHostCapability() {
  return $api.useQuery(
    "get",
    "/api/v1/host-actions/native-capability",
    {},
    { staleTime: 30_000, refetchOnWindowFocus: true },
  );
}

export function useNativeHostAction(actionID: string, enabled: boolean) {
  return $api.useQuery(
    "get",
    "/api/v1/host-actions/{id}",
    { params: { path: { id: actionID } } },
    { enabled: enabled && actionID.length > 0, refetchInterval: 1500 },
  );
}

export function useUnfinishedNativeHostActions(enabled: boolean) {
  return $api.useQuery("get", "/api/v1/host-actions", {}, { enabled, refetchOnWindowFocus: true });
}

export function useCreateNativeHostAction() {
  return $api.useMutation("post", "/api/v1/host-actions");
}

export function useResolveNativeHostAction() {
  return $api.useMutation("post", "/api/v1/host-actions/{id}/resolve");
}

export function useCancelNativeHostAction() {
  return $api.useMutation("delete", "/api/v1/host-actions/{id}");
}
