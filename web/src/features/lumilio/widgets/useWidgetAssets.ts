import { $api } from "@/lib/http-commons/queryClient";
import type { AgentRefAssetsDTO } from "../types";
import type { WidgetSource } from "./types";

/** Hydration hooks: the data plane. Both endpoints return the same page
 * shape ({assets, total, pagination}), so widgets are source-agnostic. */

export function useWidgetAssetsPreview(source: WidgetSource, limit: number) {
  const common = { retry: false, staleTime: 60_000 } as const;

  const refQuery = $api.useQuery(
    "get",
    "/api/v1/agent/refs/{id}/assets",
    {
      params: {
        path: { id: source.kind === "ref" ? source.refId : "" },
        query: {
          thread_id: source.kind === "ref" ? source.threadId : "",
          limit,
        },
      },
    },
    { ...common, enabled: source.kind === "ref" },
  );

  const pinQuery = $api.useQuery(
    "get",
    "/api/v1/agent/pins/{id}/assets",
    {
      params: {
        path: { id: source.kind === "pin" ? source.pinId : "" },
        query: { limit },
      },
    },
    { ...common, enabled: source.kind === "pin" },
  );

  const query = source.kind === "ref" ? refQuery : pinQuery;
  const payload = query.data as AgentRefAssetsDTO | undefined;
  return {
    assets: payload?.assets ?? [],
    total: payload?.total ?? 0,
    isLoading: query.isLoading,
    isError: query.isError,
  };
}

export function useWidgetAssetsInfinite(
  source: WidgetSource,
  pageSize: number,
) {
  const infiniteOptions = (enabled: boolean) => ({
    pageParamName: "offset" as const,
    initialPageParam: 0,
    retry: false,
    enabled,
    getNextPageParam: (
      lastPage: unknown,
      _allPages: unknown[],
      lastPageParam: unknown,
    ) => {
      const payload = lastPage as AgentRefAssetsDTO | undefined;
      const fetched =
        (Number(lastPageParam ?? 0) || 0) + (payload?.assets?.length ?? 0);
      return payload && fetched < (payload.total ?? 0) ? fetched : undefined;
    },
  });

  const refQuery = $api.useInfiniteQuery(
    "get",
    "/api/v1/agent/refs/{id}/assets",
    {
      params: {
        path: { id: source.kind === "ref" ? source.refId : "" },
        query: {
          thread_id: source.kind === "ref" ? source.threadId : "",
          limit: pageSize,
        },
      },
    },
    infiniteOptions(source.kind === "ref"),
  );

  const pinQuery = $api.useInfiniteQuery(
    "get",
    "/api/v1/agent/pins/{id}/assets",
    {
      params: {
        path: { id: source.kind === "pin" ? source.pinId : "" },
        query: { limit: pageSize },
      },
    },
    infiniteOptions(source.kind === "pin"),
  );

  return source.kind === "ref" ? refQuery : pinQuery;
}
