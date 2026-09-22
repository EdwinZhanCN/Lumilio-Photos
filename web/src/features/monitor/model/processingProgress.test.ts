import { describe, expect, it } from "vite-plus/test";
import {
  FILES_PER_LAYER,
  TRAY_LAYERS,
  TRAY_REFERENCE,
  trayLayers,
  trayProgress,
  trayTier,
  type TrayTier,
} from "./processingProgress";

describe("tray scale", () => {
  it("fills at the fixed reference and empties at zero pending", () => {
    expect(trayProgress(TRAY_REFERENCE)).toBe(0);
    expect(trayProgress(0)).toBe(100);
  });

  it("never rescales to the current backlog", () => {
    // Same pending count, same percentage, regardless of what came before.
    expect(trayProgress(288)).toBe(50);
    expect(trayProgress(288)).toBe(50);
  });

  it("clamps work beyond the reference instead of going negative", () => {
    expect(trayProgress(TRAY_REFERENCE * 4)).toBe(0);
    expect(trayProgress(-50)).toBe(100);
    expect(trayProgress(Number.NaN)).toBe(100);
  });

  it("uses a fixed file unit so a count reads as a stack", () => {
    expect(TRAY_REFERENCE).toBe(576);
    expect(trayProgress(FILES_PER_LAYER)).toBeCloseTo(100 - 100 / TRAY_LAYERS, 6);
  });

  it("counts pending work into layers, rounded up", () => {
    expect(trayLayers(0)).toBe(0);
    expect(trayLayers(1)).toBe(1);
    expect(trayLayers(FILES_PER_LAYER)).toBe(1);
    expect(trayLayers(FILES_PER_LAYER + 1)).toBe(2);
    expect(trayLayers(TRAY_REFERENCE)).toBe(TRAY_LAYERS);
    expect(trayLayers(TRAY_REFERENCE * 4)).toBe(TRAY_LAYERS);
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

    for (let step = 0; step <= 40; step += 1) {
      const pending = (TRAY_REFERENCE * step) / 40;
      const tier = trayTier(trayProgress(pending));
      const layers = trayLayers(pending);
      expect(bands[tier](layers), `${pending} pending → ${tier}/${layers}`).toBe(true);
    }
  });
});
