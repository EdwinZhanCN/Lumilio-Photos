import { render } from "vitest-browser-react";
import { describe, expect, it } from "vite-plus/test";
import "./App.css";

const expectedLayers = [
  { name: "sticky", className: "z-sticky", value: "100" },
  { name: "dropdown", className: "z-dropdown", value: "200" },
  { name: "overlay", className: "z-overlay", value: "300" },
  { name: "modal", className: "z-modal", value: "400" },
  { name: "lightbox", className: "z-lightbox", value: "500" },
  { name: "tooltip", className: "z-tooltip", value: "600" },
  { name: "toast", className: "z-toast", value: "700" },
] as const;

describe("z-index policy", () => {
  it("emits the global layer tokens at their documented values", async () => {
    await render(
      <div>
        {expectedLayers.map((layer) => (
          <div key={layer.name} data-layer={layer.name} className={`relative ${layer.className}`} />
        ))}
      </div>,
    );

    for (const layer of expectedLayers) {
      const element = document.querySelector<HTMLElement>(`[data-layer="${layer.name}"]`);
      expect(element, `${layer.name} layer`).not.toBeNull();
      expect(getComputedStyle(element!).zIndex).toBe(layer.value);
    }
  });

  it("overrides component-library defaults at global layer roots", async () => {
    await render(
      <div>
        <div data-layer-root="drawer" className="drawer-side z-overlay" />
        <div data-layer-root="modal" className="modal modal-open z-modal" />
        <div data-layer-root="toast" className="toast z-toast" />
        <div
          data-layer-root="inline-toast"
          className="fixed"
          style={{ zIndex: "var(--z-index-toast)" }}
        />
      </div>,
    );

    const expectedRoots = {
      drawer: "300",
      modal: "400",
      toast: "700",
      "inline-toast": "700",
    };

    for (const [root, value] of Object.entries(expectedRoots)) {
      const element = document.querySelector<HTMLElement>(`[data-layer-root="${root}"]`);
      expect(element, `${root} root`).not.toBeNull();
      expect(getComputedStyle(element!).zIndex).toBe(value);
    }
  });
});
