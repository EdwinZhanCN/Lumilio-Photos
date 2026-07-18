import {
  isAssetUserFilterFieldActive,
  normalizeAssetUserFilter,
  pickAssetUserFilter,
  type AssetLocationBBox,
  type AssetUserFilter,
  type AssetUserFilterKey,
} from "../../../model/filter";
import type { FilterDraft } from "./types";

export const EMPTY_LOCATION_BBOX: AssetLocationBBox = {
  north: 0,
  south: 0,
  east: 0,
  west: 0,
};

export function centerToBBox(lat: number, lon: number, radiusKm: number): AssetLocationBBox {
  const dLat = radiusKm / 110.574;
  const dLon = radiusKm / (111.32 * Math.cos((lat * Math.PI) / 180));
  return {
    north: lat + dLat,
    south: lat - dLat,
    east: lon + dLon,
    west: lon - dLon,
  };
}

export function isZeroBBox(bbox: AssetLocationBBox): boolean {
  return bbox.north === 0 && bbox.south === 0 && bbox.east === 0 && bbox.west === 0;
}

export function areLocationBBoxesEqual(left: AssetLocationBBox, right: AssetLocationBBox): boolean {
  return (
    left.north === right.north &&
    left.south === right.south &&
    left.east === right.east &&
    left.west === right.west
  );
}

export function toDateInput(value: string): string {
  if (!value) return "";
  if (value.includes("T")) return value.split("T")[0];
  return value;
}

export function createLockedFieldSet(
  lockedFields: readonly AssetUserFilterKey[] | undefined,
): Set<AssetUserFilterKey> {
  return new Set(lockedFields ?? []);
}

export function hasActiveLockedFields(
  initial: AssetUserFilter,
  lockedFieldSet: ReadonlySet<AssetUserFilterKey>,
): boolean {
  return Array.from(lockedFieldSet).some((field) => isAssetUserFilterFieldActive(initial, field));
}

export function buildLockedInitialFilter(
  initial: AssetUserFilter,
  lockedFieldSet: ReadonlySet<AssetUserFilterKey>,
): AssetUserFilter {
  return pickAssetUserFilter(initial, Array.from(lockedFieldSet));
}

export function mergeLockedInitialFilter(
  filter: AssetUserFilter,
  initial: AssetUserFilter,
  lockedFieldSet: ReadonlySet<AssetUserFilterKey>,
): AssetUserFilter {
  return normalizeAssetUserFilter({
    ...filter,
    ...buildLockedInitialFilter(initial, lockedFieldSet),
  });
}

export function createFilterDraft(
  initial: AssetUserFilter,
  hasLockedInitialFilters: boolean,
): FilterDraft {
  return {
    filterEnabled: Object.keys(initial).length > 0 || hasLockedInitialFilters,
    typeEnabled: initial.type === "PHOTO" || initial.type === "VIDEO",
    typeValue: initial.type === "VIDEO" ? "VIDEO" : "PHOTO",
    rawEnabled: typeof initial.raw === "boolean",
    rawMode: initial.raw === false ? "exclude" : "include",
    ratingEnabled: typeof initial.rating === "number",
    ratingValue: typeof initial.rating === "number" ? initial.rating : 5,
    likedEnabled: typeof initial.liked === "boolean",
    likedValue: initial.liked ?? true,
    filenameEnabled: Boolean(initial.filename),
    filenameOperator: initial.filename?.operator ?? "contains",
    filenameValue: initial.filename?.value ?? "",
    dateEnabled: Boolean(initial.date),
    dateFrom: toDateInput(initial.date?.from ?? ""),
    dateTo: toDateInput(initial.date?.to ?? ""),
    locationEnabled: Boolean(initial.location),
    location: initial.location ?? EMPTY_LOCATION_BBOX,
    cameraModelEnabled: Boolean(initial.camera_model),
    cameraModel: initial.camera_model ?? "",
    lensEnabled: Boolean(initial.lens),
    lens: initial.lens ?? "",
    tagEnabled: Boolean(initial.tag_names?.length),
    tagNames: initial.tag_names ?? [],
  };
}

export type FilterDraftAction =
  | { type: "replace"; draft: FilterDraft }
  | { type: "set"; key: keyof FilterDraft; value: FilterDraft[keyof FilterDraft] };

export function filterDraftReducer(state: FilterDraft, action: FilterDraftAction): FilterDraft {
  if (action.type === "replace") return action.draft;
  if (Object.is(state[action.key], action.value)) return state;
  return { ...state, [action.key]: action.value };
}

export function countEnabledFilters(draft: FilterDraft, hasLockedInitialFilters: boolean): number {
  if (!draft.filterEnabled && !hasLockedInitialFilters) return 0;

  let count = 0;
  if (draft.rawEnabled) count++;
  if (draft.typeEnabled) count++;
  if (draft.ratingEnabled) count++;
  if (draft.likedEnabled) count++;
  if (draft.filenameEnabled && draft.filenameValue.trim() !== "") count++;
  if (draft.dateEnabled && (draft.dateFrom || draft.dateTo)) count++;
  if (draft.locationEnabled && !isZeroBBox(draft.location)) count++;
  if (draft.cameraModelEnabled && draft.cameraModel) count++;
  if (draft.lensEnabled && draft.lens) count++;
  if (draft.tagEnabled && draft.tagNames.length > 0) count++;
  return count;
}

export function buildAssetUserFilter(
  draft: FilterDraft,
  initial: AssetUserFilter,
  lockedFieldSet: ReadonlySet<AssetUserFilterKey>,
  hasLockedInitialFilters: boolean,
): AssetUserFilter {
  if (!draft.filterEnabled && !hasLockedInitialFilters) return {};

  const filter: AssetUserFilter = {};
  if (draft.filterEnabled && draft.typeEnabled) filter.type = draft.typeValue;
  if (draft.filterEnabled && draft.rawEnabled) filter.raw = draft.rawMode === "include";
  if (draft.filterEnabled && draft.ratingEnabled) filter.rating = draft.ratingValue;
  if (draft.filterEnabled && draft.likedEnabled) filter.liked = draft.likedValue;
  if (draft.filterEnabled && draft.filenameEnabled && draft.filenameValue.trim()) {
    filter.filename = {
      operator: draft.filenameOperator,
      value: draft.filenameValue.trim(),
    };
  }
  if (draft.filterEnabled && draft.dateEnabled && (draft.dateFrom || draft.dateTo)) {
    filter.date = {
      from: draft.dateFrom || undefined,
      to: draft.dateTo || undefined,
    };
  }
  if (draft.filterEnabled && draft.locationEnabled && !isZeroBBox(draft.location)) {
    filter.location = { ...draft.location };
  }
  if (draft.filterEnabled && draft.cameraModelEnabled && draft.cameraModel) {
    filter.camera_model = draft.cameraModel;
  }
  if (draft.filterEnabled && draft.lensEnabled && draft.lens) filter.lens = draft.lens;
  if (draft.filterEnabled && draft.tagEnabled && draft.tagNames.length > 0) {
    filter.tag_names = draft.tagNames;
  }

  return mergeLockedInitialFilter(filter, initial, lockedFieldSet);
}
