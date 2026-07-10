import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons";

export type TagSummaryListResponse = components["schemas"]["dto.TagSummaryListResponseDTO"];

export interface UseTagSummariesOptions {
  repositoryId?: string;
  source?: string;
  query?: string;
  limit?: number;
  offset?: number;
}

/**
 * Browsable tag vocabulary (manual and AI/system) with usage counts and
 * covers, for the Tags collection view. Distinct from the autocomplete-only
 * `/api/v1/assets/tags` used elsewhere for `@`-mentions.
 */
export function useTagSummaries({
  repositoryId,
  source,
  query,
  limit = 60,
  offset = 0,
}: UseTagSummariesOptions) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/tag-summaries",
    {
      params: {
        query: {
          repository_id: repositoryId,
          source,
          q: query,
          limit,
          offset,
        },
      },
    },
  );
}
