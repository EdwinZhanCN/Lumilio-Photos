import { useCallback, useMemo } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

type PublicShareMetadataDTO = components["schemas"]["dto.PublicShareMetadataDTO"];
type PublicAssetDTO = components["schemas"]["dto.PublicAssetDTO"];
const PAGE_SIZE = 60;

export interface PublicShareViewResult {
  metadata: PublicShareMetadataDTO | undefined;
  isMetadataLoading: boolean;
  /** True once the token has been confirmed invalid/expired/revoked. */
  notFound: boolean;
  assets: PublicAssetDTO[];
  total: number;
  isLoading: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  fetchMore: () => Promise<void>;
}

/**
 * Public, tokenless-auth gallery view for one share link. Modeled on
 * usePinAssetsView's "resolve source, then paginate" shape, but deliberately
 * much smaller: v1 public shares are browse-only in date order (no
 * filter/search/sort/stacks), and the DTOs are the de-sensitized
 * PublicAssetDTO/PublicShareMetadataDTO shapes, never the full internal
 * Asset/BrowseItem types.
 */
export function usePublicShareView(token: string | undefined): PublicShareViewResult {
  const enabled = Boolean(token);

  const metadataQuery = $api.useQuery(
    "get",
    "/api/v1/public/shares/{token}",
    { params: { path: { token: token ?? "" } } },
    { enabled, retry: false },
  );

  const assetsQuery = $api.useInfiniteQuery(
    "post",
    "/api/v1/public/shares/{token}/assets/list",
    {
      params: { path: { token: token ?? "" } },
      body: { limit: PAGE_SIZE, offset: 0 },
    },
    {
      enabled: enabled && metadataQuery.isSuccess,
      initialPageParam: 0,
      pageParamName: "offset",
      retry: false,
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const total = lastPage?.total ?? 0;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = lastPage?.items?.length ?? 0;
        const hasMore = offset + loadedCount < total;
        return hasMore ? offset + PAGE_SIZE : undefined;
      },
    },
  );

  const assets = useMemo(() => {
    const pages = assetsQuery.data?.pages ?? [];
    return pages.flatMap((page) => page.items ?? []);
  }, [assetsQuery.data]);

  const total = useMemo(() => {
    const pages = assetsQuery.data?.pages ?? [];
    return pages.length > 0 ? (pages[pages.length - 1]?.total ?? 0) : 0;
  }, [assetsQuery.data]);

  const fetchMore = useCallback(async () => {
    await assetsQuery.fetchNextPage();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [assetsQuery.fetchNextPage]);

  return {
    metadata: metadataQuery.data,
    isMetadataLoading: enabled && metadataQuery.isLoading,
    notFound: enabled && metadataQuery.isError,
    assets,
    total,
    isLoading: enabled && (metadataQuery.isLoading || assetsQuery.isLoading),
    isLoadingMore: assetsQuery.isFetchingNextPage,
    hasMore: assetsQuery.hasNextPage ?? false,
    fetchMore,
  };
}
