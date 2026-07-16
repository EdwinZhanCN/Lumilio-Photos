import { describe, expect, it } from "vite-plus/test";

import {
  autoConvertCoordinates,
  convertFromGaodeCoordinates,
  convertToGaodeCoordinates,
  needsCoordinateConversion,
} from "./coordinateConversion";

describe("coordinate conversion", () => {
  it("converts Beijing WGS-84 coordinates to the expected GCJ-02 offset", () => {
    const converted = convertToGaodeCoordinates(116.4074, 39.9042);

    expect(converted.longitude).toBeCloseTo(116.413642, 6);
    expect(converted.latitude).toBeCloseTo(39.905603, 6);
    expect(needsCoordinateConversion(116.4074, 39.9042)).toBe(true);
  });

  it("approximately reverses a GCJ-02 conversion", () => {
    const converted = convertToGaodeCoordinates(116.4074, 39.9042);
    const restored = convertFromGaodeCoordinates(converted.longitude, converted.latitude);

    expect(restored.longitude).toBeCloseTo(116.4074, 4);
    expect(restored.latitude).toBeCloseTo(39.9042, 4);
  });

  it("keeps coordinates unchanged outside China or when Gaode conversion is disabled", () => {
    const london = { longitude: -0.1276, latitude: 51.5072 };

    expect(convertToGaodeCoordinates(london.longitude, london.latitude)).toEqual(london);
    expect(needsCoordinateConversion(london.longitude, london.latitude)).toBe(false);
    expect(autoConvertCoordinates(116.4074, 39.9042, false)).toEqual({
      longitude: 116.4074,
      latitude: 39.9042,
    });
  });
});
