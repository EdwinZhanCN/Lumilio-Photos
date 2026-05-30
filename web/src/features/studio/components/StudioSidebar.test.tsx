import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StudioSidebar } from "./StudioSidebar";

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

afterEach(() => {
  cleanup();
});

describe("StudioSidebar", () => {
  it("renders all nav items and highlights the active panel", () => {
    const setActivePanel = vi.fn();

    render(
      <StudioSidebar
        activePanel="exif"
        setActivePanel={setActivePanel}
      />,
    );

    expect(screen.getByText("studio.nav.exif")).toBeInTheDocument();
    expect(screen.getByText("studio.nav.develop")).toBeInTheDocument();
    expect(screen.getByText("studio.nav.border")).toBeInTheDocument();
  });

  it("calls setActivePanel when a nav item is clicked", () => {
    const setActivePanel = vi.fn();

    render(
      <StudioSidebar
        activePanel="exif"
        setActivePanel={setActivePanel}
      />,
    );

    fireEvent.click(screen.getByText("studio.nav.border"));
    expect(setActivePanel).toHaveBeenCalledWith("border");
  });
});
