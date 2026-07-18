import type { SortByType } from "../types";
import { normalizeAssetUserFilter, type AssetLocationBBox, type AssetUserFilter } from "./filter";

export type AssetBrowseRouteState = {
  query: string;
  sort: SortByType;
  filter: AssetUserFilter;
};

export const DEFAULT_ASSET_BROWSE_SORT: SortByType = "date_captured";

const BROWSE_PARAM_KEYS = [
  "q",
  "sort",
  "type",
  "raw",
  "rating",
  "liked",
  "filename",
  "filename_op",
  "from",
  "to",
  "camera",
  "lens",
  "tag",
  "bbox",
] as const;

export function hasAssetBrowseParams(params: URLSearchParams): boolean {
  return BROWSE_PARAM_KEYS.some((key) => params.has(key));
}

const parseBoolean = (value: string | null): boolean | undefined => {
  if (value === "true" || value === "1") return true;
  if (value === "false" || value === "0") return false;
  return undefined;
};

const parseRating = (value: string | null): number | undefined => {
  if (value === null || !/^\d+$/.test(value)) return undefined;
  const rating = Number(value);
  return rating >= 0 && rating <= 5 ? rating : undefined;
};

const parseDate = (value: string | null): string | undefined => {
  if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return undefined;
  const timestamp = Date.parse(`${value}T00:00:00Z`);
  return Number.isNaN(timestamp) ? undefined : value;
};

const parseBBox = (value: string | null): AssetLocationBBox | undefined => {
  if (!value) return undefined;
  const parts = value.split(",");
  if (parts.length !== 4) return undefined;

  const [west, south, east, north] = parts.map(Number);
  if (
    [west, south, east, north].some((part) => !Number.isFinite(part)) ||
    west < -180 ||
    west > 180 ||
    east < -180 ||
    east > 180 ||
    south < -90 ||
    south > 90 ||
    north < -90 ||
    north > 90 ||
    north < south
  ) {
    return undefined;
  }

  return { west, south, east, north };
};

export function parseAssetBrowseParams(
  params: URLSearchParams,
  defaultSort: SortByType = DEFAULT_ASSET_BROWSE_SORT,
): AssetBrowseRouteState {
  const typeParam = params.get("type")?.toLocaleLowerCase();
  const type = typeParam === "photo" ? "PHOTO" : typeParam === "video" ? "VIDEO" : undefined;
  const filename = params.get("filename")?.trim();
  const filenameOperator = params.get("filename_op");
  const operator =
    filenameOperator === "contains" ||
    filenameOperator === "matches" ||
    filenameOperator === "starts_with" ||
    filenameOperator === "ends_with"
      ? filenameOperator
      : "contains";
  const from = parseDate(params.get("from"));
  const to = parseDate(params.get("to"));

  const filter = normalizeAssetUserFilter({
    type,
    raw: parseBoolean(params.get("raw")),
    rating: parseRating(params.get("rating")),
    liked: parseBoolean(params.get("liked")),
    filename: filename ? { operator, value: filename } : undefined,
    date: from || to ? { from, to } : undefined,
    camera_model: params.get("camera") ?? undefined,
    lens: params.get("lens") ?? undefined,
    tag_names: params.getAll("tag"),
    location: parseBBox(params.get("bbox")),
  });

  return {
    query: params.get("q")?.trim() ?? "",
    sort: params.get("sort") === "recently_added" ? "recently_added" : defaultSort,
    filter,
  };
}

export function serializeAssetBrowseParams(
  state: AssetBrowseRouteState,
  current: URLSearchParams = new URLSearchParams(),
  defaultSort: SortByType = DEFAULT_ASSET_BROWSE_SORT,
): URLSearchParams {
  const params = new URLSearchParams(current);
  BROWSE_PARAM_KEYS.forEach((key) => params.delete(key));

  const query = state.query.trim();
  if (query) params.set("q", query);
  if (state.sort !== defaultSort) params.set("sort", state.sort);

  const filter = normalizeAssetUserFilter(state.filter);
  if (filter.type) params.set("type", filter.type.toLocaleLowerCase());
  if (typeof filter.raw === "boolean") params.set("raw", String(filter.raw));
  if (typeof filter.rating === "number") params.set("rating", String(filter.rating));
  if (typeof filter.liked === "boolean") params.set("liked", String(filter.liked));
  if (filter.filename) {
    params.set("filename", filter.filename.value);
    if (filter.filename.operator !== "contains") {
      params.set("filename_op", filter.filename.operator);
    }
  }
  if (filter.date?.from) params.set("from", filter.date.from);
  if (filter.date?.to) params.set("to", filter.date.to);
  if (filter.camera_model) params.set("camera", filter.camera_model);
  if (filter.lens) params.set("lens", filter.lens);
  filter.tag_names?.forEach((tag) => params.append("tag", tag));
  if (filter.location) {
    params.set(
      "bbox",
      [
        filter.location.west,
        filter.location.south,
        filter.location.east,
        filter.location.north,
      ].join(","),
    );
  }

  return params;
}
