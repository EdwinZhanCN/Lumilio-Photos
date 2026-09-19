import { describe, expect, it, vi } from "vite-plus/test";
import { userEvent } from "vitest/browser";
import { renderWithProviders } from "@test/render";
import { AnchoredMenu, MenuItem } from "./AnchoredMenu";

function renderInClippingParent(onSelect: () => void) {
  return renderWithProviders(
    // Mirrors the storage page: a rounded section is overflow-hidden and the
    // table inside scrolls horizontally, so a positioned child would be clipped.
    <div data-testid="clipper" className="overflow-hidden rounded-box">
      <div className="overflow-x-auto">
        <AnchoredMenu label="Actions for Family Archive">
          {({ close }) => (
            <MenuItem
              onSelect={() => {
                close();
                onSelect();
              }}
            >
              Verify now
            </MenuItem>
          )}
        </AnchoredMenu>
      </div>
    </div>,
  );
}

describe("AnchoredMenu", () => {
  it("renders the menu outside a clipping ancestor", async () => {
    const screen = await renderInClippingParent(vi.fn());

    await screen.getByRole("button", { name: "Actions for Family Archive" }).click();

    const menu = document.querySelector('[role="menu"]');
    expect(menu).not.toBeNull();
    const clipper = document.querySelector('[data-testid="clipper"]');
    // The whole point: the menu escapes the overflow-hidden ancestor instead of
    // being cut off by it.
    expect(clipper?.contains(menu!)).toBe(false);
    expect(document.body.contains(menu!)).toBe(true);
    await expect.element(screen.getByRole("menuitem", { name: "Verify now" })).toBeVisible();
  });

  it("closes after choosing an item and reports the command", async () => {
    const onSelect = vi.fn();
    const screen = await renderInClippingParent(onSelect);

    await screen.getByRole("button", { name: "Actions for Family Archive" }).click();
    await screen.getByRole("menuitem", { name: "Verify now" }).click();

    expect(onSelect).toHaveBeenCalledTimes(1);
    await vi.waitFor(() => expect(document.querySelector('[role="menu"]')).toBeNull());
  });

  it("closes on an outside click and on Escape", async () => {
    const screen = await renderInClippingParent(vi.fn());
    const trigger = screen.getByRole("button", { name: "Actions for Family Archive" });

    await trigger.click();
    expect(document.querySelector('[role="menu"]')).not.toBeNull();
    await userEvent.click(document.body);
    await vi.waitFor(() => expect(document.querySelector('[role="menu"]')).toBeNull());

    await trigger.click();
    expect(document.querySelector('[role="menu"]')).not.toBeNull();
    await userEvent.keyboard("{Escape}");
    await vi.waitFor(() => expect(document.querySelector('[role="menu"]')).toBeNull());
  });
});
