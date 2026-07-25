import { describe, expect, it } from "vite-plus/test";
import { initializeJustifiedLayout, JustifiedLayout } from "./justifiedWasm";

describe("JustifiedLayout WASM bridge", () => {
  it("initializes the embedded module before a worker reports ready", () => {
    expect(() => initializeJustifiedLayout()).not.toThrow();
  });

  it("returns the package layout for a row of square assets", () => {
    const layout = new JustifiedLayout(new Float32Array([1, 1, 1]), {
      rowHeight: 100,
      rowWidth: 320,
      spacing: 10,
      heightTolerance: 0.15,
    });

    expect(layout.containerWidth).toBeCloseTo(320);
    expect(layout.containerHeight).toBeCloseTo(100);
    expect(layout.getPosition(0)).toEqual({
      top: expect.closeTo(0),
      left: expect.closeTo(0),
      width: expect.closeTo(100),
      height: expect.closeTo(100),
    });
    expect(layout.getPosition(2)).toEqual({
      top: expect.closeTo(0),
      left: expect.closeTo(220),
      width: expect.closeTo(100),
      height: expect.closeTo(100),
    });
  });

  it("returns an empty container", () => {
    const layout = new JustifiedLayout(new Float32Array(), {
      rowHeight: 100,
      rowWidth: 320,
      spacing: 10,
      heightTolerance: 0.15,
    });

    expect(layout.containerWidth).toBe(0);
    expect(layout.containerHeight).toBe(0);
  });
});
