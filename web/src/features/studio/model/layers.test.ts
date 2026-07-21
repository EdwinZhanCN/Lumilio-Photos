import { describe, expect, it } from "vite-plus/test";
import {
  createLogoLayer,
  createTextLayer,
  displayText,
  normalizeLayers,
  type TextLayer,
} from "./layers";

describe("createTextLayer", () => {
  it("centers a new layer and leaves it unstyled", () => {
    const layer = createTextLayer();
    expect(layer.x).toBe(0.5);
    expect(layer.y).toBe(0.5);
    expect(layer.shadow).toBeNull();
    expect(layer.stroke).toBeNull();
    expect(layer.fromTemplate).toBe(false);
  });

  it("gives every layer a distinct id", () => {
    expect(createTextLayer().id).not.toBe(createTextLayer().id);
  });
});

describe("displayText", () => {
  const layer = (overrides: Partial<TextLayer>) => createTextLayer({ text: "hello world", ...overrides });

  it("applies the case transform", () => {
    expect(displayText(layer({ textCase: "upper" }))).toBe("HELLO WORLD");
    expect(displayText(layer({ textCase: "title" }))).toBe("Hello World");
    expect(displayText(layer({ textCase: "lower", text: "HELLO" }))).toBe("hello");
    expect(displayText(layer({ textCase: "none" }))).toBe("hello world");
  });
});

describe("normalizeLayers", () => {
  it("returns an empty stack for anything that is not an array", () => {
    expect(normalizeLayers(undefined)).toEqual([]);
    expect(normalizeLayers({ layers: [] })).toEqual([]);
  });

  it("drops entries it cannot understand instead of rendering something wrong", () => {
    const layers = normalizeLayers([
      null,
      { type: "text", text: "keep" },
      { type: "wormhole" },
      { type: "logo" }, // no brand
      "nonsense",
    ]);
    expect(layers).toHaveLength(1);
    expect(layers[0].type).toBe("text");
  });

  it("preserves draw order", () => {
    const layers = normalizeLayers([
      { type: "text", text: "first" },
      { type: "logo", brand: "canon" },
      { type: "text", text: "third" },
    ]);
    expect(layers.map((layer) => layer.type)).toEqual(["text", "logo", "text"]);
  });

  it("fills defaults for a sparse text layer", () => {
    const [layer] = normalizeLayers([{ type: "text", text: "hi" }]);
    expect(layer).toMatchObject({ type: "text", x: 0.5, y: 0.5, opacity: 1 });
    if (layer.type !== "text") throw new Error("expected a text layer");
    expect(layer.font.family).toBeTruthy();
    expect(layer.fill.kind).toBe("solid");
  });

  it("clamps hostile numbers", () => {
    const [layer] = normalizeLayers([
      { type: "text", text: "x", opacity: 42, rotation: 1e9, font: { size: -3, weight: 5000 } },
    ]);
    if (layer.type !== "text") throw new Error("expected a text layer");
    expect(layer.opacity).toBe(1);
    expect(layer.rotation).toBe(360);
    expect(layer.font.size).toBeGreaterThan(0);
    expect(layer.font.weight).toBe(900);
  });

  it("treats a logo colour of null as 'keep the original'", () => {
    const [layer] = normalizeLayers([{ type: "logo", brand: "leica", variant: "symbol" }]);
    if (layer.type !== "logo") throw new Error("expected a logo layer");
    expect(layer.color).toBeNull();
  });

  it("round-trips a layer it produced itself", () => {
    const original = createTextLayer({
      text: "X-T5",
      align: "left",
      x: 0.12,
      y: 0.9,
      shadow: { color: "#000000", opacity: 0.4, blur: 0.01, offsetX: 0, offsetY: 0.002 },
    });
    const [restored] = normalizeLayers([JSON.parse(JSON.stringify(original))]);
    expect(restored).toEqual(original);
  });

  it("round-trips a logo layer", () => {
    const original = createLogoLayer("fujifilm", "wordmark", { size: 0.2, x: 0.8, y: 0.94 });
    const [restored] = normalizeLayers([JSON.parse(JSON.stringify(original))]);
    expect(restored).toEqual(original);
  });
});
