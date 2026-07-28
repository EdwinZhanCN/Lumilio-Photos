import type { components } from "@/lib/http-commons";

type AssetFilterDTO = components["schemas"]["dto.AssetFilterDTO"];
type AssetType = NonNullable<AssetFilterDTO["type"]>;
type FilenameOperator = NonNullable<NonNullable<AssetFilterDTO["filename"]>["operator"]>;

export type MediaComposition = NonNullable<
  NonNullable<AssetFilterDTO["media_item"]>["composition"]
>;
export type StackMembership = NonNullable<NonNullable<AssetFilterDTO["stack"]>["membership"]>;
export type StackKind = NonNullable<NonNullable<AssetFilterDTO["stack"]>["kinds"]>[number];

export const MEDIA_COMPOSITIONS = [
  "contains_raw",
  "jpeg_raw",
  "raw_unpaired",
  "no_raw",
  "live_photo",
] as const satisfies readonly MediaComposition[];

export const STACK_MEMBERSHIPS = [
  "stacked",
  "unstacked",
] as const satisfies readonly StackMembership[];

export const STACK_KINDS = ["burst", "manual"] as const satisfies readonly StackKind[];

export type AssetBrowseConstraint = AssetFilterDTO;

export type AssetLocationBBox = {
  north: number;
  south: number;
  east: number;
  west: number;
};

export type AssetUserFilter = {
  type?: Extract<AssetType, "PHOTO" | "VIDEO">;
  media_item?: {
    composition?: MediaComposition;
  };
  stack?: {
    membership?: StackMembership;
    kinds?: StackKind[];
  };
  rating?: number;
  liked?: boolean;
  filename?: {
    operator: FilenameOperator;
    value: string;
  };
  date?: {
    from?: string;
    to?: string;
  };
  camera_model?: string;
  lens?: string;
  tag_names?: string[];
  location?: AssetLocationBBox;
};

export type AssetUserFilterKey = keyof AssetUserFilter;

/**
 * Order matters for display only — active-filter chips render in this order, so
 * it mirrors the filter panel's section order and the two never disagree.
 * Counting, stripping and picking treat these keys as a set.
 */
export const ASSET_USER_FILTER_KEYS = [
  "type",
  "liked",
  "rating",
  "media_item",
  "stack",
  "filename",
  "date",
  "location",
  "camera_model",
  "lens",
  "tag_names",
] as const satisfies readonly AssetUserFilterKey[];

const FILENAME_OPERATORS = new Set<FilenameOperator>([
  "contains",
  "matches",
  "starts_with",
  "ends_with",
]);

const MEDIA_COMPOSITION_SET: ReadonlySet<string> = new Set(MEDIA_COMPOSITIONS);
const STACK_MEMBERSHIP_SET: ReadonlySet<string> = new Set(STACK_MEMBERSHIPS);
const STACK_KIND_SET: ReadonlySet<string> = new Set(STACK_KINDS);

export const isMediaComposition = (value: unknown): value is MediaComposition =>
  typeof value === "string" && MEDIA_COMPOSITION_SET.has(value);

export const isStackMembership = (value: unknown): value is StackMembership =>
  typeof value === "string" && STACK_MEMBERSHIP_SET.has(value);

export const isStackKind = (value: unknown): value is StackKind =>
  typeof value === "string" && STACK_KIND_SET.has(value);

const normalizeStackKinds = (kinds: readonly string[] | undefined): StackKind[] | undefined => {
  if (!kinds) return undefined;

  const seen = new Set<StackKind>();
  kinds.forEach((kind) => {
    if (isStackKind(kind)) seen.add(kind);
  });

  return seen.size > 0 ? STACK_KINDS.filter((kind) => seen.has(kind)) : undefined;
};

const trimOptional = (value: string | undefined): string | undefined => {
  const normalized = value?.trim();
  return normalized ? normalized : undefined;
};

const normalizeTags = (tags: string[] | undefined): string[] | undefined => {
  if (!tags) return undefined;

  const seen = new Set<string>();
  const normalized = tags.flatMap((tag) => {
    const value = tag.trim();
    const identity = value.toLocaleLowerCase();
    if (!value || seen.has(identity)) return [];
    seen.add(identity);
    return [value];
  });

  return normalized.length > 0 ? normalized : undefined;
};

export const isCompleteLocationBBox = (
  location: AssetFilterDTO["location"] | AssetLocationBBox | undefined,
): location is AssetLocationBBox =>
  Boolean(
    location &&
    typeof location.north === "number" &&
    Number.isFinite(location.north) &&
    typeof location.south === "number" &&
    Number.isFinite(location.south) &&
    typeof location.east === "number" &&
    Number.isFinite(location.east) &&
    typeof location.west === "number" &&
    Number.isFinite(location.west),
  );

export function normalizeAssetUserFilter(filter: AssetUserFilter): AssetUserFilter {
  const normalized: AssetUserFilter = {};

  if (filter.type === "PHOTO" || filter.type === "VIDEO") normalized.type = filter.type;

  const composition = filter.media_item?.composition;
  if (isMediaComposition(composition)) normalized.media_item = { composition };

  const membership = isStackMembership(filter.stack?.membership)
    ? filter.stack.membership
    : undefined;
  // Backend rejects `unstacked` together with kinds, so an unstacked selection drops kinds.
  const stackKinds =
    membership === "unstacked" ? undefined : normalizeStackKinds(filter.stack?.kinds);
  if (membership || stackKinds) {
    normalized.stack = {
      ...(membership ? { membership } : {}),
      ...(stackKinds ? { kinds: stackKinds } : {}),
    };
  }

  if (
    typeof filter.rating === "number" &&
    Number.isInteger(filter.rating) &&
    filter.rating >= 0 &&
    filter.rating <= 5
  ) {
    normalized.rating = filter.rating;
  }
  if (typeof filter.liked === "boolean") normalized.liked = filter.liked;

  const filename = trimOptional(filter.filename?.value);
  const filenameOperator = filter.filename?.operator;
  if (filename && filenameOperator && FILENAME_OPERATORS.has(filenameOperator)) {
    normalized.filename = { operator: filenameOperator, value: filename };
  }

  const from = trimOptional(filter.date?.from);
  const to = trimOptional(filter.date?.to);
  if (from || to) normalized.date = { from, to };

  const cameraModel = trimOptional(filter.camera_model);
  if (cameraModel) normalized.camera_model = cameraModel;

  const lens = trimOptional(filter.lens);
  if (lens) normalized.lens = lens;

  const tags = normalizeTags(filter.tag_names);
  if (tags) normalized.tag_names = tags;

  if (isCompleteLocationBBox(filter.location)) {
    normalized.location = { ...filter.location };
  }

  return normalized;
}

export function isAssetUserFilterFieldActive(
  filter: AssetUserFilter | AssetBrowseConstraint,
  key: AssetUserFilterKey,
): boolean {
  switch (key) {
    case "type":
      return filter.type === "PHOTO" || filter.type === "VIDEO";
    case "media_item":
      return isMediaComposition(filter.media_item?.composition);
    case "stack":
      return Boolean(
        isStackMembership(filter.stack?.membership) ||
        filter.stack?.kinds?.some((kind) => isStackKind(kind)),
      );
    case "liked":
      return typeof filter[key] === "boolean";
    case "rating":
      return typeof filter.rating === "number";
    case "filename":
      return Boolean(filter.filename?.value?.trim());
    case "date":
      return Boolean(filter.date?.from || filter.date?.to);
    case "camera_model":
    case "lens":
      return Boolean(filter[key]?.trim());
    case "tag_names":
      return Boolean(filter.tag_names?.some((tag) => tag.trim()));
    case "location":
      return isCompleteLocationBBox(filter.location);
  }
}

export function getConstrainedFilterKeys(
  constraint: AssetBrowseConstraint | undefined,
): ReadonlySet<AssetUserFilterKey> {
  if (!constraint) return new Set();

  return new Set(
    ASSET_USER_FILTER_KEYS.filter((key) => isAssetUserFilterFieldActive(constraint, key)),
  );
}

export function getConstraintUserFilter(
  constraint: AssetBrowseConstraint | undefined,
): AssetUserFilter {
  if (!constraint) return {};

  return normalizeAssetUserFilter({
    type: constraint.type === "PHOTO" || constraint.type === "VIDEO" ? constraint.type : undefined,
    media_item: constraint.media_item,
    stack: constraint.stack,
    rating: constraint.rating,
    liked: constraint.liked,
    filename:
      constraint.filename?.operator && constraint.filename.value
        ? {
            operator: constraint.filename.operator,
            value: constraint.filename.value,
          }
        : undefined,
    date: constraint.date,
    camera_model: constraint.camera_model,
    lens: constraint.lens,
    tag_names: constraint.tag_names,
    location: isCompleteLocationBBox(constraint.location) ? { ...constraint.location } : undefined,
  });
}

export function countActiveAssetUserFilters(filter: AssetUserFilter): number {
  const normalized = normalizeAssetUserFilter(filter);
  return ASSET_USER_FILTER_KEYS.filter((key) => isAssetUserFilterFieldActive(normalized, key))
    .length;
}

export function mergeAssetFilters(
  userFilter: AssetUserFilter,
  constraint: AssetBrowseConstraint | undefined,
): AssetFilterDTO {
  return {
    ...normalizeAssetUserFilter(userFilter),
    ...constraint,
  };
}

export function stripConstrainedAssetUserFilter(
  filter: AssetUserFilter,
  constraint: AssetBrowseConstraint | undefined,
): AssetUserFilter {
  const normalized = normalizeAssetUserFilter(filter);
  const constrainedKeys = getConstrainedFilterKeys(constraint);

  return Object.fromEntries(
    Object.entries(normalized).filter(([key]) => !constrainedKeys.has(key as AssetUserFilterKey)),
  ) as AssetUserFilter;
}

export function pickAssetUserFilter(
  filter: AssetUserFilter,
  keys: readonly AssetUserFilterKey[],
): AssetUserFilter {
  const normalized = normalizeAssetUserFilter(filter);
  const allowed = new Set(keys);
  return Object.fromEntries(
    Object.entries(normalized).filter(([key]) => allowed.has(key as AssetUserFilterKey)),
  ) as AssetUserFilter;
}
