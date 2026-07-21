import { describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { RootErrorFallback } from "./RootErrorBoundary";

describe("RootErrorFallback", () => {
  it("offers recovery without depending on the router", async () => {
    const resetErrorBoundary = vi.fn();

    const screen = await renderWithProviders(
      <RootErrorFallback error={new Error("render failed")} resetErrorBoundary={resetErrorBoundary} />,
      { router: false },
    );

    await expect
      .element(screen.getByRole("heading", { name: "Lumilio could not continue" }))
      .toBeVisible();
    await expect.element(screen.getByText("render failed")).toBeVisible();
    await expect
      .element(screen.getByRole("link", { name: "Return home" }))
      .toHaveAttribute("href", "/");

    await screen.getByRole("button", { name: "Reload application" }).click();
    expect(resetErrorBoundary).toHaveBeenCalledOnce();
  });
});
