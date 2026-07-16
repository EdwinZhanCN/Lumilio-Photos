import { describe, expect, it } from "vite-plus/test";

import { formatGPSCoordinates } from "./formatCoordinates";

describe("formatGPSCoordinates", () => {
  it("formats northern and eastern coordinates with the requested precision", () => {
    expect(formatGPSCoordinates(39.9042, 116.4074, 4)).toBe("39.9042°N, 116.4074°E");
  });

  it("formats southern and western coordinates as positive magnitudes", () => {
    expect(formatGPSCoordinates(-33.8688, -151.2093, 2)).toBe("33.87°S, 151.21°W");
  });
});
