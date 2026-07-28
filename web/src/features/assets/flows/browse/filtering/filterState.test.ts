import { describe, expect, it } from "vite-plus/test";
import {
  buildAssetUserFilter,
  buildLockedInitialFilter,
  centerToBBox,
  createFilterDraft,
  createLockedFieldSet,
  EMPTY_LOCATION_BBOX,
  isZeroBBox,
  toDateInput,
} from "./filterState";
import { isAssetUserFilterFieldActive, type AssetUserFilter } from "../../../model/filter";
import type { FilterDraft } from "./types";

const createDraft = (overrides: Partial<FilterDraft> = {}): FilterDraft => ({
  stackKinds: [],
  filenameOperator: "contains",
  filenameValue: "",
  dateFrom: "",
  dateTo: "",
  cameraModel: "",
  lens: "",
  tagNames: [],
  ...overrides,
});

describe("FilterTool filter state", () => {
  it("treats boolean false and rating zero as active filter values", () => {
    const dto: AssetUserFilter = { liked: false, rating: 0 };

    expect(isAssetUserFilterFieldActive(dto, "liked")).toBe(true);
    expect(isAssetUserFilterFieldActive(dto, "rating")).toBe(true);
  });

  it("treats composition and stack selections as active filter values", () => {
    expect(
      isAssetUserFilterFieldActive({ media_item: { composition: "no_raw" } }, "media_item"),
    ).toBe(true);
    expect(isAssetUserFilterFieldActive({ stack: { membership: "unstacked" } }, "stack")).toBe(
      true,
    );
    expect(isAssetUserFilterFieldActive({ stack: { kinds: ["burst"] } }, "stack")).toBe(true);
    expect(isAssetUserFilterFieldActive({ stack: { kinds: [] } }, "stack")).toBe(false);
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
  });

  it("seeds a flat draft from an existing filter", () => {
    expect(
      createFilterDraft({
        type: "VIDEO",
        media_item: { composition: "jpeg_raw" },
        stack: { membership: "stacked", kinds: ["burst"] },
        rating: 0,
        liked: false,
        date: { from: "2026-07-16T13:14:15Z" },
      }),
    ).toEqual(
      createDraft({
        type: "VIDEO",
        composition: "jpeg_raw",
        stackMembership: "stacked",
        stackKinds: ["burst"],
        rating: 0,
        liked: false,
        dateFrom: "2026-07-16",
      }),
    );
  });

  it("lets locked initial values override the editable draft", () => {
    const initial: AssetUserFilter = { type: "PHOTO", liked: true };
    const lockedFields = createLockedFieldSet(["type"]);
    const draft = createDraft({ type: "VIDEO", liked: false });

    expect(buildAssetUserFilter(draft, initial, lockedFields)).toEqual({
      type: "PHOTO",
      liked: false,
    });
  });

  it("keeps locked filters that the draft never carries", () => {
    const initial: AssetUserFilter = { media_item: { composition: "no_raw" } };
    const lockedFields = createLockedFieldSet(["media_item"]);

    expect(buildAssetUserFilter(createDraft(), initial, lockedFields)).toEqual({
      media_item: { composition: "no_raw" },
    });
  });

  it("builds composition and stack filters from the draft", () => {
    expect(
      buildAssetUserFilter(
        createDraft({ composition: "raw_unpaired", stackKinds: ["manual"] }),
        {},
        new Set(),
      ),
    ).toEqual({
      media_item: { composition: "raw_unpaired" },
      stack: { kinds: ["manual"] },
    });
  });

  it("trims filename filters and omits values left empty", () => {
    const draft = createDraft({
      filenameValue: "  beach  ",
      dateTo: "2026-07-16",
      location: EMPTY_LOCATION_BBOX,
    });

    expect(buildAssetUserFilter(draft, {}, new Set())).toEqual({
      filename: { operator: "contains", value: "beach" },
      date: { from: undefined, to: "2026-07-16" },
    });
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
