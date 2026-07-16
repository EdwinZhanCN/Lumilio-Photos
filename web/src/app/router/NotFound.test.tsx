import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import NotFound from "./NotFound";

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({ t: (_key: string, fallback: string) => fallback }),
}));

describe("NotFound", () => {
  it("provides home and history recovery actions", () => {
    render(
      <MemoryRouter initialEntries={["/previous", "/missing-page"]} initialIndex={1}>
        <Routes>
          <Route path="/previous" element={<p>Previous page</p>} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "This page does not exist" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Go to library" })).toHaveAttribute("href", "/");

    fireEvent.click(screen.getByRole("button", { name: "Go back" }));
    expect(screen.getByText("Previous page")).toBeVisible();
  });
});
