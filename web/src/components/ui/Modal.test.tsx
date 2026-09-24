import { describe, expect, it, vi } from "vite-plus/test";
import { userEvent } from "vitest/browser";
import { renderWithProviders } from "@test/render";
import { Modal } from "./Modal";

async function renderModal(props: Partial<Parameters<typeof Modal>[0]> = {}) {
  const onClose = vi.fn();
  const screen = await renderWithProviders(
    <Modal open onClose={onClose} title="Backup Location" {...props}>
      <p>Body content</p>
    </Modal>,
  );
  return { onClose, screen };
}

describe("Modal", () => {
  it("renders a native dialog labelled by its title", async () => {
    const { screen } = await renderModal();

    const dialog = screen.getByRole("dialog", { name: "Backup Location" });
    await expect.element(dialog).toBeVisible();
    // The dialog role and the accessible name come from the <dialog> element
    // plus aria-labelledby, not from a hand-rolled container div.
    await expect.element(dialog).toHaveAttribute("open");
    await expect.element(screen.getByText("Body content")).toBeVisible();
  });

  it("moves focus into the dialog", async () => {
    const { screen } = await renderModal();
    const dialog = screen.getByRole("dialog", { name: "Backup Location" });

    await expect.element(dialog).toBeVisible();
    expect(dialog.element().contains(document.activeElement)).toBe(true);
  });

  it("closes on Escape", async () => {
    const { onClose } = await renderModal();
    await userEvent.keyboard("{Escape}");
    await vi.waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it("does not close on Escape when dismissal is disabled", async () => {
    const { onClose } = await renderModal({ dismissable: false });
    await userEvent.keyboard("{Escape}");
    // Give the browser a turn to deliver a cancel event before asserting.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes on a backdrop click but not on a click inside the box", async () => {
    const { onClose, screen } = await renderModal();
    const dialog = screen.getByRole("dialog", { name: "Backup Location" });
    await expect.element(dialog).toBeVisible();

    await screen.getByText("Body content").click();
    expect(onClose).not.toHaveBeenCalled();

    // A click that lands on the dialog element itself (outside modal-box) is
    // the backdrop click.
    dialog.element().dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await vi.waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it("mounts children only while open", async () => {
    const screen = await renderWithProviders(
      <Modal open={false} onClose={vi.fn()} title="Closed">
        <p>Body content</p>
      </Modal>,
    );

    expect(screen.container.textContent).not.toContain("Body content");
  });
});
