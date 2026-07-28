import { describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import { TypeSection } from "./ChoiceSections";

// Covers the reset's behaviour, not its layout: the gap this component was
// changed to fix is a CSS effect and is not observable here.
describe("ChoiceSections pills", () => {
  const clearLabel = t("assets.filterTool.common.clear_choice");

  it("renders no clear control while nothing is selected", async () => {
    const screen = await renderWithProviders(
      <TypeSection locked={false} value={undefined} onValueChange={vi.fn()} />,
      { router: false },
    );

    await expect.element(screen.getByRole("button", { name: clearLabel })).not.toBeInTheDocument();
    await expect
      .element(screen.getByRole("radio", { name: t("assets.filterTool.typeSection.photo") }))
      .toBeVisible();
  });

  it("clears the selection through the reset control", async () => {
    const onValueChange = vi.fn();
    const screen = await renderWithProviders(
      <TypeSection locked={false} value="PHOTO" onValueChange={onValueChange} />,
      { router: false },
    );

    await screen.getByRole("button", { name: clearLabel }).click();

    expect(onValueChange).toHaveBeenCalledWith(undefined);
  });

  it("selects a value and offers no reset while the field is locked", async () => {
    const onValueChange = vi.fn();
    const screen = await renderWithProviders(
      <TypeSection locked={false} value={undefined} onValueChange={onValueChange} />,
      { router: false },
    );

    await screen.getByRole("radio", { name: t("assets.filterTool.typeSection.video") }).click();
    expect(onValueChange).toHaveBeenCalledWith("VIDEO");

    const locked = await renderWithProviders(
      <TypeSection locked value="PHOTO" onValueChange={vi.fn()} />,
      { router: false },
    );
    await expect.element(locked.getByRole("button", { name: clearLabel })).not.toBeInTheDocument();
  });
});
