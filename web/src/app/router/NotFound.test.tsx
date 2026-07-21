import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import NotFound from "./NotFound";

describe("NotFound", () => {
  it("provides home and history recovery actions", async () => {
    // Own the router here so history has a real previous entry for "Go back";
    // renderWithProviders still supplies the real i18n, context and query providers.
    const screen = await renderWithProviders(
      <MemoryRouter initialEntries={["/previous", "/missing-page"]} initialIndex={1}>
        <Routes>
          <Route path="/previous" element={<p>Previous page</p>} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </MemoryRouter>,
      { router: false },
    );

    await expect
      .element(screen.getByRole("heading", { name: t("notFound.title") }))
      .toBeVisible();
    await expect
      .element(screen.getByRole("link", { name: t("notFound.home") }))
      .toHaveAttribute("href", "/");

    await screen.getByRole("button", { name: t("notFound.back") }).click();
    await expect.element(screen.getByText("Previous page")).toBeVisible();
  });
});
