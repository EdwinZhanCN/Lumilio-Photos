import {
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

export function createFilterDraft(initial: AssetUserFilter): FilterDraft {
  const normalized = normalizeAssetUserFilter(initial);

  return {
    type: normalized.type,
    composition: normalized.media_item?.composition,
    stackMembership: normalized.stack?.membership,
    stackKinds: normalized.stack?.kinds ?? [],
    rating: normalized.rating,
    liked: normalized.liked,
    filenameOperator: normalized.filename?.operator ?? "contains",
    filenameValue: normalized.filename?.value ?? "",
    dateFrom: toDateInput(normalized.date?.from ?? ""),
    dateTo: toDateInput(normalized.date?.to ?? ""),
    location: normalized.location,
    cameraModel: normalized.camera_model ?? "",
    lens: normalized.lens ?? "",
    tagNames: normalized.tag_names ?? [],
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

export function buildAssetUserFilter(
  draft: FilterDraft,
  initial: AssetUserFilter,
  lockedFieldSet: ReadonlySet<AssetUserFilterKey>,
): AssetUserFilter {
  const filter: AssetUserFilter = {};
  if (draft.type) filter.type = draft.type;
  if (draft.composition) filter.media_item = { composition: draft.composition };
  if (draft.stackMembership || draft.stackKinds.length > 0) {
    filter.stack = {
      ...(draft.stackMembership ? { membership: draft.stackMembership } : {}),
      ...(draft.stackKinds.length > 0 ? { kinds: draft.stackKinds } : {}),
    };
  }
  if (typeof draft.rating === "number") filter.rating = draft.rating;
  if (typeof draft.liked === "boolean") filter.liked = draft.liked;
  if (draft.filenameValue.trim()) {
    filter.filename = {
      operator: draft.filenameOperator,
      value: draft.filenameValue.trim(),
    };
  }
  if (draft.dateFrom || draft.dateTo) {
    filter.date = {
      from: draft.dateFrom || undefined,
      to: draft.dateTo || undefined,
    };
  }
  if (draft.location && !isZeroBBox(draft.location)) {
    filter.location = { ...draft.location };
  }
  if (draft.cameraModel) filter.camera_model = draft.cameraModel;
  if (draft.lens) filter.lens = draft.lens;
  if (draft.tagNames.length > 0) filter.tag_names = draft.tagNames;

  return mergeLockedInitialFilter(filter, initial, lockedFieldSet);
}
