import { $api } from "@/lib/http-commons/queryClient";

export const browserCapabilitiesQueryKey = ["get", "/api/v1/auth/browser-capabilities"] as const;

export function useBrowserCapabilities() {
  return $api.useQuery(
    "get",
    "/api/v1/auth/browser-capabilities",
    {},
    {
      staleTime: Infinity,
      refetchOnWindowFocus: false,
      retry: false,
    },
  );
}
