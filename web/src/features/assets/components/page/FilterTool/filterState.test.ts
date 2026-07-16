import { describe, expect, it } from "vite-plus/test";
import {
  buildFilterDTO,
  buildLockedInitialDTO,
  centerToBBox,
  countEnabledFilters,
  createLockedFieldSet,
  EMPTY_LOCATION_BBOX,
  hasActiveLockedFields,
  isFieldActive,
  toDateInput,
} from "./filterState";
import type { FilterDraft, FilterDTO } from "./types";

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
    const dto: FilterDTO = { raw: false, liked: false, rating: 0 };

    expect(isFieldActive(dto, "raw")).toBe(true);
    expect(isFieldActive(dto, "liked")).toBe(true);
    expect(isFieldActive(dto, "rating")).toBe(true);
  });

  it("rejects empty text, empty tags, and the zero location box", () => {
    const dto: FilterDTO = {
      filename: { operator: "contains", value: "   " },
      camera_model: " ",
      lens: " ",
      tag_names: [],
      location: EMPTY_LOCATION_BBOX,
    };

    expect(isFieldActive(dto, "filename")).toBe(false);
    expect(isFieldActive(dto, "camera_model")).toBe(false);
    expect(isFieldActive(dto, "lens")).toBe(false);
    expect(isFieldActive(dto, "tag_names")).toBe(false);
    expect(isFieldActive(dto, "location")).toBe(false);
  });

  it("normalizes locked values and only keeps active locked fields", () => {
    const initial: FilterDTO = {
      filename: { operator: "starts_with", value: "  IMG_ " },
      camera_model: "  Leica M11  ",
      lens: " ",
      tag_names: ["travel"],
    };
    const lockedFields = createLockedFieldSet({
      filename: true,
      camera_model: true,
      lens: true,
      tag_names: true,
      raw: false,
    });

    expect(buildLockedInitialDTO(initial, lockedFields)).toEqual({
      filename: { operator: "starts_with", value: "IMG_" },
      camera_model: "Leica M11",
      tag_names: ["travel"],
    });
    expect(hasActiveLockedFields(initial, lockedFields)).toBe(true);
  });

  it("lets locked initial values override the editable draft", () => {
    const initial: FilterDTO = { type: "PHOTO", liked: true };
    const lockedFields = createLockedFieldSet(["type"]);
    const draft = createDraft({
      typeEnabled: true,
      typeValue: "VIDEO",
      likedEnabled: true,
      likedValue: false,
    });

    expect(buildFilterDTO(draft, initial, lockedFields, true)).toEqual({
      type: "PHOTO",
      liked: false,
    });
  });

  it("keeps locked filters when the global editable filter switch is off", () => {
    const initial: FilterDTO = { raw: false };
    const lockedFields = createLockedFieldSet(["raw"]);
    const draft = createDraft({ filterEnabled: false });

    expect(buildFilterDTO(draft, initial, lockedFields, true)).toEqual({ raw: false });
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

    expect(buildFilterDTO(draft, {}, new Set(), false)).toEqual({
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
