import { $api } from "@/lib/http-commons/queryClient";
import type { AgentRefDTO } from "../../types";
import { getMockWidgetDataset, isMockWidgetSource } from "./mockWidgetData";
import type { AgentPinDTO, WidgetSource } from "./types";

export function useWidgetMetadata(source: WidgetSource) {
  const common = { retry: false, staleTime: 60_000 } as const;
  const mockDataset = isMockWidgetSource(source) ? getMockWidgetDataset(source.mockId) : undefined;

  const refQuery = $api.useQuery(
    "get",
    "/api/v1/agent/refs/{id}",
    {
      params: {
        path: { id: source.kind === "ref" ? source.refId : "" },
        query: {
          thread_id: source.kind === "ref" ? source.threadId : "",
        },
      },
    },
    { ...common, enabled: source.kind === "ref" },
  );

  const pinQuery = $api.useQuery(
    "get",
    "/api/v1/agent/pins/{id}",
    {
      params: {
        path: { id: source.kind === "pin" ? source.pinId : "" },
      },
    },
    { ...common, enabled: source.kind === "pin" },
  );

  if (mockDataset) {
    return {
      metadata: mockDataset.metadata,
      facets: mockDataset.metadata.facets,
      isLoading: false,
      isError: false,
    };
  }

  const query = source.kind === "ref" ? refQuery : pinQuery;
  const payload: AgentRefDTO | AgentPinDTO | undefined = query.data;

  return {
    metadata: payload,
    facets: payload?.facets,
    isLoading: query.isLoading,
    isError: query.isError,
  };
}
