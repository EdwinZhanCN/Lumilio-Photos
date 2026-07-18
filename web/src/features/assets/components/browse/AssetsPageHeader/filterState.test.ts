import { describe, expect, it } from "vite-plus/test";
import { filterDTOToPayload, filtersToDTO } from "./filterState";

describe("AssetsPageHeader filter state", () => {
  it("maps persisted filename modes to the FilterTool contract", () => {
    expect(
      filtersToDTO({
        enabled: true,
        filename: { mode: "startswith", value: "IMG_" },
      }),
    ).toEqual({
      filename: { operator: "starts_with", value: "IMG_" },
    });
  });

  it("builds a full reset payload while normalizing FilterTool values", () => {
    expect(
      filterDTOToPayload({
        filename: { operator: "ends_with", value: " .jpg " },
        camera_model: " Camera ",
        tag_names: ["family"],
      }),
    ).toEqual({
      enabled: true,
      type: undefined,
      raw: undefined,
      rating: undefined,
      liked: undefined,
      filename: { mode: "endswith", value: ".jpg" },
      date: undefined,
      camera_model: "Camera",
      lens: undefined,
      tag_names: ["family"],
      location: undefined,
    });
  });

  it("disables filters when FilterTool emits an empty DTO", () => {
    expect(filterDTOToPayload({})).toEqual({
      enabled: false,
      type: undefined,
      raw: undefined,
      rating: undefined,
      liked: undefined,
      filename: undefined,
      date: undefined,
      camera_model: undefined,
      lens: undefined,
      tag_names: undefined,
      location: undefined,
    });
  });
});
