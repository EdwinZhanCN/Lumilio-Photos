import type { FilterDraft, FilterDTO, FilterFieldKey, LocationBBox } from "./types";

export const EMPTY_LOCATION_BBOX: LocationBBox = {
  north: 0,
  south: 0,
  east: 0,
  west: 0,
};

export function centerToBBox(lat: number, lon: number, radiusKm: number): LocationBBox {
  const dLat = radiusKm / 110.574;
  const dLon = radiusKm / (111.32 * Math.cos((lat * Math.PI) / 180));
  return {
    north: lat + dLat,
    south: lat - dLat,
    east: lon + dLon,
    west: lon - dLon,
  };
}

export function isZeroBBox(bbox: LocationBBox): boolean {
  return bbox.north === 0 && bbox.south === 0 && bbox.east === 0 && bbox.west === 0;
}

export function areLocationBBoxesEqual(left: LocationBBox, right: LocationBBox): boolean {
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

export function isFieldActive(dto: FilterDTO, field: FilterFieldKey): boolean {
  switch (field) {
    case "type":
      return dto.type === "PHOTO" || dto.type === "VIDEO";
    case "raw":
      return typeof dto.raw === "boolean";
    case "rating":
      return typeof dto.rating === "number";
    case "liked":
      return typeof dto.liked === "boolean";
    case "filename":
      return !!dto.filename?.value?.trim();
    case "date":
      return !!dto.date && (!!dto.date.from || !!dto.date.to);
    case "camera_model":
      return !!dto.camera_model?.trim();
    case "lens":
      return !!dto.lens?.trim();
    case "tag_names":
      return !!dto.tag_names && dto.tag_names.length > 0;
    case "location":
      return !!dto.location && !isZeroBBox(dto.location);
  }
}

export function createLockedFieldSet(
  lockedFields: readonly FilterFieldKey[] | Partial<Record<FilterFieldKey, boolean>> | undefined,
): Set<FilterFieldKey> {
  if (!lockedFields) return new Set<FilterFieldKey>();
  if (Array.isArray(lockedFields)) return new Set<FilterFieldKey>(lockedFields);

  return new Set<FilterFieldKey>(
    (Object.entries(lockedFields) as [FilterFieldKey, boolean | undefined][])
      .filter(([, locked]) => locked)
      .map(([field]) => field),
  );
}

export function hasActiveLockedFields(
  initial: FilterDTO,
  lockedFieldSet: ReadonlySet<FilterFieldKey>,
): boolean {
  return Array.from(lockedFieldSet).some((field) => isFieldActive(initial, field));
}

export function buildLockedInitialDTO(
  initial: FilterDTO,
  lockedFieldSet: ReadonlySet<FilterFieldKey>,
): FilterDTO {
  const dto: FilterDTO = {};

  if (lockedFieldSet.has("type") && isFieldActive(initial, "type")) dto.type = initial.type;
  if (lockedFieldSet.has("raw") && isFieldActive(initial, "raw")) dto.raw = initial.raw;
  if (lockedFieldSet.has("rating") && isFieldActive(initial, "rating")) {
    dto.rating = initial.rating;
  }
  if (lockedFieldSet.has("liked") && isFieldActive(initial, "liked")) {
    dto.liked = initial.liked;
  }
  if (lockedFieldSet.has("filename") && isFieldActive(initial, "filename")) {
    dto.filename = {
      operator: initial.filename!.operator,
      value: initial.filename!.value.trim(),
    };
  }
  if (lockedFieldSet.has("date") && isFieldActive(initial, "date")) {
    dto.date = { from: initial.date!.from, to: initial.date!.to };
  }
  if (lockedFieldSet.has("camera_model") && isFieldActive(initial, "camera_model")) {
    dto.camera_model = initial.camera_model!.trim();
  }
  if (lockedFieldSet.has("lens") && isFieldActive(initial, "lens")) {
    dto.lens = initial.lens!.trim();
  }
  if (lockedFieldSet.has("tag_names") && isFieldActive(initial, "tag_names")) {
    dto.tag_names = [...initial.tag_names!];
  }
  if (lockedFieldSet.has("location") && isFieldActive(initial, "location")) {
    dto.location = { ...initial.location! };
  }

  return dto;
}

export function mergeLockedInitialDTO(
  dto: FilterDTO,
  initial: FilterDTO,
  lockedFieldSet: ReadonlySet<FilterFieldKey>,
): FilterDTO {
  return { ...dto, ...buildLockedInitialDTO(initial, lockedFieldSet) };
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

export function buildFilterDTO(
  draft: FilterDraft,
  initial: FilterDTO,
  lockedFieldSet: ReadonlySet<FilterFieldKey>,
  hasLockedInitialFilters: boolean,
): FilterDTO {
  if (!draft.filterEnabled && !hasLockedInitialFilters) return {};

  const dto: FilterDTO = {};
  if (draft.filterEnabled && draft.typeEnabled) dto.type = draft.typeValue;
  if (draft.filterEnabled && draft.rawEnabled) dto.raw = draft.rawMode === "include";
  if (draft.filterEnabled && draft.ratingEnabled) dto.rating = draft.ratingValue;
  if (draft.filterEnabled && draft.likedEnabled) dto.liked = draft.likedValue;
  if (draft.filterEnabled && draft.filenameEnabled && draft.filenameValue.trim()) {
    dto.filename = {
      operator: draft.filenameOperator,
      value: draft.filenameValue.trim(),
    };
  }
  if (draft.filterEnabled && draft.dateEnabled && (draft.dateFrom || draft.dateTo)) {
    dto.date = {
      from: draft.dateFrom || undefined,
      to: draft.dateTo || undefined,
    };
  }
  if (draft.filterEnabled && draft.locationEnabled && !isZeroBBox(draft.location)) {
    dto.location = { ...draft.location };
  }
  if (draft.filterEnabled && draft.cameraModelEnabled && draft.cameraModel) {
    dto.camera_model = draft.cameraModel;
  }
  if (draft.filterEnabled && draft.lensEnabled && draft.lens) dto.lens = draft.lens;
  if (draft.filterEnabled && draft.tagEnabled && draft.tagNames.length > 0) {
    dto.tag_names = draft.tagNames;
  }

  return mergeLockedInitialDTO(dto, initial, lockedFieldSet);
}
