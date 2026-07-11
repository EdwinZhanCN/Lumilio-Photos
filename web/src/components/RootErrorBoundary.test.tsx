import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RootErrorFallback } from "./RootErrorBoundary";

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({ t: (_key: string, fallback: string) => fallback }),
}));

describe("RootErrorFallback", () => {
  it("offers recovery without depending on the router", () => {
    const resetErrorBoundary = vi.fn();

    render(
      <RootErrorFallback
        error={new Error("render failed")}
        resetErrorBoundary={resetErrorBoundary}
      />,
    );

    expect(screen.getByRole("heading", { name: "Lumilio could not continue" })).toBeVisible();
    expect(screen.getByText("render failed")).toBeVisible();
    expect(screen.getByRole("link", { name: "Return home" })).toHaveAttribute("href", "/");

    fireEvent.click(screen.getByRole("button", { name: "Reload application" }));
    expect(resetErrorBoundary).toHaveBeenCalledOnce();
  });
});
