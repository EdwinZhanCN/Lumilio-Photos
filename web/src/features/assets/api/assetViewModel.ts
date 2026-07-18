import type { Asset } from "@/lib/assets/types";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { AssetMediaType, AssetsViewResult, AssetViewDefinition, BrowseGroup } from "../types";
import type { AssetBrowseConstraint } from "../model/filter";

type SearchAssetsRequest = components["schemas"]["dto.SearchAssetsRequestDTO"];
type SearchAssetsResponse = components["schemas"]["dto.SearchAssetsResponseDTO"];

export type SearchTopResultsMeta = {
  enabled: boolean;
  degraded: boolean;
  reason?: string;
  source_types: string[];
};

export type AssetBrowserViewResult = AssetsViewResult & {
  topResults: Asset[];
  resultAssets: Asset[];
  resultGroups: { key: string; assets: Asset[] }[];
  topResultsBrowseGroups: BrowseGroup[];
  resultBrowseGroups: BrowseGroup[];
  topResultsMeta: SearchTopResultsMeta;
};

export const DEFAULT_TOP_RESULTS_META: SearchTopResultsMeta = {
  enabled: false,
  degraded: false,
  source_types: [],
};

export const TOP_RESULTS_LIMIT = 9;
export const DEFAULT_ASSET_TYPES: AssetMediaType[] = ["photos", "videos"];

function getApiMimeTypes(mediaTypes: AssetMediaType[]): ("PHOTO" | "VIDEO" | "AUDIO")[] {
  return mediaTypes.map((type) => {
    if (type === "photos") return "PHOTO";
    if (type === "videos") return "VIDEO";
    return "AUDIO";
  });
}

export function mergeUniqueAssets(...collections: Asset[][]): Asset[] {
  const seen = new Set<string>();
  return collections.flatMap((collection) =>
    collection.filter((asset) => {
      if (!asset.asset_id || seen.has(asset.asset_id)) return false;
      seen.add(asset.asset_id);
      return true;
    }),
  );
}

export function normalizeTopResultsMeta(
  meta?: SearchAssetsResponse["top_results_meta"],
): SearchTopResultsMeta {
  return {
    enabled: Boolean(meta?.enabled),
    degraded: Boolean(meta?.degraded),
    reason: meta?.reason,
    source_types: meta?.source_types ?? [],
  };
}

export function normalizeAssetSort(
  sortBy?: AssetViewDefinition["sortBy"],
): SearchAssetsRequest["sort_by"] {
  return sortBy === "recently_added" ? "recently_added" : "date_captured";
}

export function buildAssetApiFilter(
  definition: AssetViewDefinition,
  effectiveFilter: AssetBrowseConstraint,
): AssetBrowseConstraint {
  const filter: AssetBrowseConstraint = { ...effectiveFilter };
  if (filter.type === undefined && filter.types === undefined && definition.types?.length) {
    const mimeTypes = getApiMimeTypes(definition.types);
    if (mimeTypes.length === 1) filter.type = mimeTypes[0];
    else if (mimeTypes.length > 1) filter.types = mimeTypes;
  }
  return filter;
}
