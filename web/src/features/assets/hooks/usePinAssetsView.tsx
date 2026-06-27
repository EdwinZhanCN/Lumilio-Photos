import { useMemo } from "react";
import type {
  InfiniteData,
  UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { Asset } from "@/lib/assets/types";
import type {
  AssetGroup,
  AssetsViewResult,
  BrowseGroup,
  BrowseItem,
} from "@/features/assets/types/assets.type";
import {
  createBrowseGroupsFromAssets,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  mergeAdjacentBrowseGroups,
} from "@/features/assets/utils/browseItems";

type AgentPinDTO = components["schemas"]["dto.AgentPinDTO"];
type AgentRefAssetsDTO = components["schemas"]["dto.AgentRefAssetsDTO"];

const PAGE_SIZE = 60;
const PIN_GROUP_KEY = "flat:pin";

export interface PinAssetsViewResult extends AssetsViewResult {
  pin: AgentPinDTO | undefined;
  isExpired: boolean;
}

const EMPTY_VIEW: PinAssetsViewResult = {
  assets: [],
  groups: undefined,
  browseGroups: [],
  browseItems: [],
  browseAssets: [],
  isLoading: false,
  isLoadingMore: false,
  isFetched: false,
  error: null,
  fetchMore: async () => {},
  refetch: async () => {},
  hasMore: false,
  viewKey: "",
  pageInfo: { page: 1 },
  pin: undefined,
  isExpired: false,
};

/** Pin-driven gallery view. Hydrates a saved agent result (a pin) through the
 * agent pin endpoints and adapts the raw asset pages into the same
 * {@link AssetsViewResult} shape the gallery renders. A failed pin metadata
 * lookup (e.g. 404/expiry) is surfaced as {@link PinAssetsViewResult.isExpired}
 * so the page can render an expiry notice instead of a generic load error. */
export function usePinAssetsView(
  pinId: string | undefined,
): PinAssetsViewResult {
  const enabled = Boolean(pinId);

  const pinMetaQuery = $api.useQuery(
    "get",
    "/api/v1/agent/pins/{id}",
    { params: { path: { id: pinId ?? "" } } },
    { enabled, retry: false, staleTime: 60_000 },
  );

  const assetsQuery = $api.useInfiniteQuery(
    "get",
    "/api/v1/agent/pins/{id}/assets",
    { params: { path: { id: pinId ?? "" }, query: { limit: PAGE_SIZE } } },
    {
      enabled,
      initialPageParam: 0,
      pageParamName: "offset",
      retry: false,
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const payload = lastPage as AgentRefAssetsDTO | undefined;
        const offset = Number(lastPageParam ?? 0) || 0;
        const fetched = offset + (payload?.assets?.length ?? 0);
        return payload && fetched < (payload.total ?? 0) ? fetched : undefined;
      },
    },
  ) as UseInfiniteQueryResult<InfiniteData<AgentRefAssetsDTO>, unknown>;

  const pin = pinMetaQuery.data as AgentPinDTO | undefined;
  const isExpired = enabled && pinMetaQuery.isError;

  const browseGroups = useMemo<BrowseGroup[]>(() => {
    if (!enabled) return [];
    const pages = (assetsQuery.data?.pages ?? []) as AgentRefAssetsDTO[];
    if (pages.length === 0) return [];
    const perPage = pages.map((page) =>
      createBrowseGroupsFromAssets(
        (page.assets ?? []) as Asset[],
        PIN_GROUP_KEY,
      ),
    );
    return mergeAdjacentBrowseGroups(...perPage);
  }, [enabled, assetsQuery.data]);

  const browseItems = useMemo<BrowseItem[]>(
    () => flattenBrowseGroups(browseGroups),
    [browseGroups],
  );
  const browseAssets = useMemo<Asset[]>(
    () => flattenBrowseGroupsToAssets(browseGroups),
    [browseGroups],
  );
  const groups = useMemo<AssetGroup[]>(
    () =>
      browseGroups.map((group) => ({
        key: group.key,
        assets: group.items.map(getBrowseItemAsset),
      })),
    [browseGroups],
  );
  const assets = browseAssets;

  const errorMessage = useMemo<string | null>(() => {
    if (!enabled) return null;
    const queryError = assetsQuery.error ?? pinMetaQuery.error;
    if (!queryError) return null;
    if (queryError instanceof Error) return queryError.message;
    if (typeof queryError === "string") return queryError;
    return JSON.stringify(queryError) ?? "Unknown error";
  }, [enabled, assetsQuery.error, pinMetaQuery.error]);

  const pageInfo = useMemo(() => {
    const pageParams = assetsQuery.data?.pageParams ?? [];
    const pages = (assetsQuery.data?.pages ?? []) as AgentRefAssetsDTO[];
    const lastPage = pages[pages.length - 1];
    const offset = Number(pageParams[pageParams.length - 1] ?? 0) || 0;
    return {
      page: Math.floor(offset / PAGE_SIZE) + 1,
      total: lastPage?.total,
    };
  }, [assetsQuery.data]);

  if (!enabled) {
    return EMPTY_VIEW;
  }

  return {
    assets,
    groups,
    browseGroups,
    browseItems,
    browseAssets,
    isLoading: assetsQuery.isLoading,
    isLoadingMore: assetsQuery.isFetchingNextPage,
    isFetched: assetsQuery.isFetched,
    error: errorMessage,
    fetchMore: async () => {
      await assetsQuery.fetchNextPage();
    },
    refetch: async () => {
      await Promise.all([pinMetaQuery.refetch(), assetsQuery.refetch()]);
    },
    hasMore: assetsQuery.hasNextPage ?? false,
    viewKey: `assets:pin:${pinId}`,
    pageInfo,
    pin,
    isExpired,
  };
}
