import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { CopyButton } from "./Buttons";

describe("CopyButton", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows success only after text was copied", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const screen = await renderWithProviders(<CopyButton text="secret" />);

    await screen.getByRole("button", { name: "Copy" }).click();

    await expect.element(screen.getByRole("button", { name: "Copied" })).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledWith("secret");
  });

  it("reports failure when both clipboard mechanisms are unavailable", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: vi.fn(() => false),
    });
    const screen = await renderWithProviders(<CopyButton text="secret" />);

    await screen.getByRole("button", { name: "Copy" }).click();

    await expect.element(screen.getByRole("button", { name: "Copy failed." })).toBeInTheDocument();
  });
});
