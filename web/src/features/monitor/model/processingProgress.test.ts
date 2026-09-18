import { describe, expect, it } from "vite-plus/test";
import {
  LAYER_UNITS,
  TRAY_LAYERS,
  trayLayers,
  trayProgress,
  trayReference,
  trayTier,
  type TrayTier,
  type WorkType,
} from "./processingProgress";

describe("tray scale", () => {
  it("fills at the fixed reference and empties at zero pending", () => {
    expect(trayProgress(trayReference("assets"), "assets")).toBe(0);
    expect(trayProgress(0, "assets")).toBe(100);
  });

  it("never rescales to the current backlog", () => {
    // Same pending count, same percentage, regardless of what came before.
    expect(trayProgress(288, "assets")).toBe(50);
    expect(trayProgress(288, "assets")).toBe(50);
  });

  it("clamps work beyond the reference instead of going negative", () => {
    expect(trayProgress(trayReference("assets") * 4, "assets")).toBe(0);
    expect(trayProgress(-50, "assets")).toBe(100);
    expect(trayProgress(Number.NaN, "assets")).toBe(100);
  });

  it("uses a per-type unit so a count reads as a stack", () => {
    expect(trayReference("repositories")).toBe(TRAY_LAYERS);
    expect(trayReference("projections")).toBe(TRAY_LAYERS * 2);
    expect(trayProgress(LAYER_UNITS.assets, "assets")).toBeCloseTo(100 - 100 / TRAY_LAYERS, 6);
  });

  it("counts pending work into layers, rounded up", () => {
    expect(trayLayers(0, "assets")).toBe(0);
    expect(trayLayers(1, "assets")).toBe(1);
    expect(trayLayers(LAYER_UNITS.assets, "assets")).toBe(1);
    expect(trayLayers(LAYER_UNITS.assets + 1, "assets")).toBe(2);
    expect(trayLayers(trayReference("assets"), "assets")).toBe(TRAY_LAYERS);
    expect(trayLayers(trayReference("assets") * 4, "assets")).toBe(TRAY_LAYERS);
  });
});

describe("trayTier", () => {
  it("maps the asset's four thresholds", () => {
    expect(trayTier(0)).toBe("full");
    expect(trayTier(100 / 3 - 0.01)).toBe("full");
    expect(trayTier(100 / 3)).toBe("twoThirds");
    expect(trayTier(200 / 3 - 0.01)).toBe("twoThirds");
    expect(trayTier(200 / 3)).toBe("oneThird");
    expect(trayTier(99.99)).toBe("oneThird");
    expect(trayTier(100)).toBe("empty");
  });

  it("agrees with the layer count for every tier band", () => {
    // A tier is a band of layer counts: full 13–18, two thirds 7–12, one third
    // 1–6, empty 0. If these ever disagree, the tray contradicts its own text.
    const bands: Record<TrayTier, (layers: number) => boolean> = {
      full: (layers) => layers >= 13 && layers <= TRAY_LAYERS,
      twoThirds: (layers) => layers >= 7 && layers <= 12,
      oneThird: (layers) => layers >= 1 && layers <= 6,
      empty: (layers) => layers === 0,
    };

    for (const type of Object.keys(LAYER_UNITS) as WorkType[]) {
      const reference = trayReference(type);
      for (let step = 0; step <= 40; step += 1) {
        const pending = (reference * step) / 40;
        const tier = trayTier(trayProgress(pending, type));
        const layers = trayLayers(pending, type);
        expect(bands[tier](layers), `${type}: ${pending} pending → ${tier}/${layers}`).toBe(true);
      }
    }
  });
});
