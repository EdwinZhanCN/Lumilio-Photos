import { describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { ProcessingTray } from "./ProcessingTray";

describe("ProcessingTray", () => {
  it("loads the asset and fills the tray at the reference backlog", async () => {
    // 576 pending files is the tray reference for the 32-files-per-layer unit.
    const screen = await renderWithProviders(<ProcessingTray pending={576} />);

    const figure = screen.container.querySelector("figure");
    expect(figure?.dataset.tier).toBe("full");
    expect(figure?.dataset.animated).toBe("true");

    // The load is asynchronous; this is the proof the real asset rendered rather
    // than merely that a canvas was requested.
    await vi.waitFor(() => {
      expect(screen.container.querySelector("figure")?.dataset.loaded).toBe("true");
    });
    await vi.waitFor(() => {
      const canvas = screen.container.querySelector("canvas");
      const context = canvas?.getContext("2d");
      const pixels = context?.getImageData(0, 0, canvas!.width, canvas!.height).data;
      expect(pixels?.some((value, index) => index % 4 === 3 && value > 0)).toBe(true);
    });
    expect(screen.container.querySelector("figure")?.dataset.layers).toBe("18");
  });

  it("advances the tier as pending work drains", async () => {
    const screen = await renderWithProviders(<ProcessingTray pending={288} />);
    const figure = screen.container.querySelector("figure");
    // Half the 576-file reference: past a third, so one tier has been extracted.
    expect(figure?.dataset.tier).toBe("twoThirds");
    expect(figure?.dataset.layers).toBe("9");
  });

  it("shows an empty tray only when no work is left", async () => {
    const screen = await renderWithProviders(<ProcessingTray pending={0} />);
    const figure = screen.container.querySelector("figure");
    expect(figure?.dataset.tier).toBe("empty");
  });

  it("preserves backlog geometry when the asset cannot load", async () => {
    const screen = await renderWithProviders(
      <ProcessingTray pending={288} assetUrl="/missing/asset.riv" />,
    );
    const figure = screen.container.querySelector("figure");

    // The runtime reports the failure asynchronously, so the fallback is awaited.
    await vi.waitFor(() => {
      expect(screen.container.querySelector("figure")?.dataset.animated).toBe("false");
    });
    expect(screen.container.querySelector("canvas")).toBeNull();
    // The static tray still shows the same tier and layer count…
    expect(figure?.dataset.tier).toBe("twoThirds");
    expect(figure?.dataset.layers).toBe("9");
    expect(screen.container.querySelectorAll(".monitor-tray-sheet[data-visible=true]").length).toBe(
      9,
    );
  });
});
