import { describe, expect, it } from "vite-plus/test";
import {
  buildAssetUserFilter,
  buildLockedInitialFilter,
  centerToBBox,
  countEnabledFilters,
  createLockedFieldSet,
  EMPTY_LOCATION_BBOX,
  hasActiveLockedFields,
  isZeroBBox,
  toDateInput,
} from "./filterState";
import { isAssetUserFilterFieldActive, type AssetUserFilter } from "../../../model/filter";
import type { FilterDraft } from "./types";

const createDraft = (overrides: Partial<FilterDraft> = {}): FilterDraft => ({
  filterEnabled: true,
  typeEnabled: false,
  typeValue: "PHOTO",
  rawEnabled: false,
  rawMode: "include",
  ratingEnabled: false,
  ratingValue: 5,
  likedEnabled: false,
  likedValue: true,
  filenameEnabled: false,
  filenameOperator: "contains",
  filenameValue: "",
  dateEnabled: false,
  dateFrom: "",
  dateTo: "",
  locationEnabled: false,
  location: EMPTY_LOCATION_BBOX,
  cameraModelEnabled: false,
  cameraModel: "",
  lensEnabled: false,
  lens: "",
  tagEnabled: false,
  tagNames: [],
  ...overrides,
});

describe("FilterTool filter state", () => {
  it("treats boolean false and rating zero as active filter values", () => {
    const dto: AssetUserFilter = { raw: false, liked: false, rating: 0 };

    expect(isAssetUserFilterFieldActive(dto, "raw")).toBe(true);
    expect(isAssetUserFilterFieldActive(dto, "liked")).toBe(true);
    expect(isAssetUserFilterFieldActive(dto, "rating")).toBe(true);
  });

  it("rejects empty text, empty tags, and the zero location box", () => {
    const dto: AssetUserFilter = {
      filename: { operator: "contains", value: "   " },
      camera_model: " ",
      lens: " ",
      tag_names: [],
      location: EMPTY_LOCATION_BBOX,
    };

    expect(isAssetUserFilterFieldActive(dto, "filename")).toBe(false);
    expect(isAssetUserFilterFieldActive(dto, "camera_model")).toBe(false);
    expect(isAssetUserFilterFieldActive(dto, "lens")).toBe(false);
    expect(isAssetUserFilterFieldActive(dto, "tag_names")).toBe(false);
    expect(isZeroBBox(dto.location!)).toBe(true);
  });

  it("normalizes locked values and only keeps active locked fields", () => {
    const initial: AssetUserFilter = {
      filename: { operator: "starts_with", value: "  IMG_ " },
      camera_model: "  Leica M11  ",
      lens: " ",
      tag_names: ["travel"],
    };
    const lockedFields = createLockedFieldSet(["filename", "camera_model", "lens", "tag_names"]);

    expect(buildLockedInitialFilter(initial, lockedFields)).toEqual({
      filename: { operator: "starts_with", value: "IMG_" },
      camera_model: "Leica M11",
      tag_names: ["travel"],
    });
    expect(hasActiveLockedFields(initial, lockedFields)).toBe(true);
  });

  it("lets locked initial values override the editable draft", () => {
    const initial: AssetUserFilter = { type: "PHOTO", liked: true };
    const lockedFields = createLockedFieldSet(["type"]);
    const draft = createDraft({
      typeEnabled: true,
      typeValue: "VIDEO",
      likedEnabled: true,
      likedValue: false,
    });

    expect(buildAssetUserFilter(draft, initial, lockedFields, true)).toEqual({
      type: "PHOTO",
      liked: false,
    });
  });

  it("keeps locked filters when the global editable filter switch is off", () => {
    const initial: AssetUserFilter = { raw: false };
    const lockedFields = createLockedFieldSet(["raw"]);
    const draft = createDraft({ filterEnabled: false });

    expect(buildAssetUserFilter(draft, initial, lockedFields, true)).toEqual({ raw: false });
    expect(countEnabledFilters(draft, true)).toBe(0);
  });

  it("trims filename filters and omits incomplete enabled values", () => {
    const draft = createDraft({
      filenameEnabled: true,
      filenameValue: "  beach  ",
      dateEnabled: true,
      dateTo: "2026-07-16",
      locationEnabled: true,
      cameraModelEnabled: true,
      lensEnabled: true,
      tagEnabled: true,
    });

    expect(buildAssetUserFilter(draft, {}, new Set(), false)).toEqual({
      filename: { operator: "contains", value: "beach" },
      date: { from: undefined, to: "2026-07-16" },
    });
    expect(countEnabledFilters(draft, false)).toBe(2);
  });

  it("normalizes ISO dates and computes the same center-radius bounding box", () => {
    expect(toDateInput("2026-07-16T13:14:15Z")).toBe("2026-07-16");
    expect(toDateInput("2026-07-16")).toBe("2026-07-16");

    const bbox = centerToBBox(40, -74, 5);
    expect(bbox.north).toBeCloseTo(40 + 5 / 110.574);
    expect(bbox.south).toBeCloseTo(40 - 5 / 110.574);
    expect((bbox.east + bbox.west) / 2).toBeCloseTo(-74);
  });
});
