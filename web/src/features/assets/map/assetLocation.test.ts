import { describe, expect, it } from "vite-plus/test";

import type { Asset } from "@/lib/http-commons";
import { assetToPhotoLocation } from "./assetLocation";

describe("assetToPhotoLocation", () => {
  it("maps photo GPS metadata into the shared map location shape", () => {
    const asset = {
      asset_id: "asset-1",
      type: "PHOTO",
      original_filename: "beijing.jpg",
      specific_metadata: {
        description: "City view",
        gps_latitude: 39.9042,
        gps_longitude: 116.4074,
      },
    } as Asset;

    expect(assetToPhotoLocation(asset)).toMatchObject({
      id: "asset-1",
      position: [39.9042, 116.4074],
      title: "beijing.jpg",
      description: "City view",
      asset,
    });
  });

  it("rejects non-photo assets and photos without complete GPS metadata", () => {
    const video = { type: "VIDEO", specific_metadata: {} } as Asset;
    const photoWithoutLongitude = {
      type: "PHOTO",
      specific_metadata: { gps_latitude: 39.9042 },
    } as Asset;

    expect(assetToPhotoLocation(video)).toBeNull();
    expect(assetToPhotoLocation(photoWithoutLongitude)).toBeNull();
  });
});
