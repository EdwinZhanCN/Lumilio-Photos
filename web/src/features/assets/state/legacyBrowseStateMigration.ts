import {
  ASSETS_STATE_STORAGE_KEY,
  ASSETS_STATE_STORAGE_VERSION,
  LEGACY_ASSETS_STATE_STORAGE_KEY,
} from "@/lib/settings/registry";
import { isRecord, readVersionedStorageCandidate, removeStorageKeys } from "@/lib/settings/storage";
import { normalizeAssetUserFilter, type AssetUserFilter } from "../model/filter";
import { DEFAULT_ASSET_BROWSE_SORT, type AssetBrowseRouteState } from "../model/browseRouteState";

const LEGACY_FILENAME_OPERATORS = {
  contains: "contains",
  matches: "matches",
  startswith: "starts_with",
  endswith: "ends_with",
  starts_with: "starts_with",
  ends_with: "ends_with",
} as const;

function legacyFilter(candidate: unknown): AssetUserFilter {
  if (!isRecord(candidate) || candidate.enabled === false) return {};

  const filename = isRecord(candidate.filename) ? candidate.filename : undefined;
  const filenameMode = typeof filename?.mode === "string" ? filename.mode : undefined;
  const filenameOperator = filenameMode
    ? LEGACY_FILENAME_OPERATORS[filenameMode as keyof typeof LEGACY_FILENAME_OPERATORS]
    : undefined;
  const date = isRecord(candidate.date) ? candidate.date : undefined;
  const location = isRecord(candidate.location) ? candidate.location : undefined;

  return normalizeAssetUserFilter({
    type: candidate.type === "PHOTO" || candidate.type === "VIDEO" ? candidate.type : undefined,
    raw: typeof candidate.raw === "boolean" ? candidate.raw : undefined,
    rating: typeof candidate.rating === "number" ? candidate.rating : undefined,
    liked: typeof candidate.liked === "boolean" ? candidate.liked : undefined,
    filename:
      filenameOperator && typeof filename?.value === "string"
        ? { operator: filenameOperator, value: filename.value }
        : undefined,
    date:
      typeof date?.from === "string" || typeof date?.to === "string"
        ? {
            from: typeof date.from === "string" ? date.from : undefined,
            to: typeof date.to === "string" ? date.to : undefined,
          }
        : undefined,
    camera_model: typeof candidate.camera_model === "string" ? candidate.camera_model : undefined,
    lens: typeof candidate.lens === "string" ? candidate.lens : undefined,
    tag_names: Array.isArray(candidate.tag_names)
      ? candidate.tag_names.filter((tag): tag is string => typeof tag === "string")
      : undefined,
    location:
      location &&
      typeof location.north === "number" &&
      typeof location.south === "number" &&
      typeof location.east === "number" &&
      typeof location.west === "number"
        ? {
            north: location.north,
            south: location.south,
            east: location.east,
            west: location.west,
          }
        : undefined,
  });
}

export function convertLegacyAssetBrowseState(candidate: unknown): AssetBrowseRouteState {
  const root = isRecord(candidate) ? candidate : {};
  const ui = isRecord(root.ui) ? root.ui : {};
  return {
    query: typeof ui.searchQuery === "string" ? ui.searchQuery.trim() : "",
    sort: ui.sortBy === "recently_added" ? "recently_added" : DEFAULT_ASSET_BROWSE_SORT,
    filter: legacyFilter(root.filters),
  };
}

export function readLegacyAssetBrowseState(): AssetBrowseRouteState | null {
  const result = readVersionedStorageCandidate({
    key: ASSETS_STATE_STORAGE_KEY,
    version: ASSETS_STATE_STORAGE_VERSION,
    legacyKeys: [LEGACY_ASSETS_STATE_STORAGE_KEY],
  });
  return result.source === "none" ? null : convertLegacyAssetBrowseState(result.candidate);
}

export function clearLegacyAssetBrowseState(): void {
  removeStorageKeys([ASSETS_STATE_STORAGE_KEY, LEGACY_ASSETS_STATE_STORAGE_KEY]);
}
