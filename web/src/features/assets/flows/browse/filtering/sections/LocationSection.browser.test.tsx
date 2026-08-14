import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { LocationSection } from "./LocationSection";

describe("LocationSection secure-context handling", () => {
  const secureContextDescriptor = Object.getOwnPropertyDescriptor(window, "isSecureContext");

  afterEach(() => {
    if (secureContextDescriptor) {
      Object.defineProperty(window, "isSecureContext", secureContextDescriptor);
    } else {
      Reflect.deleteProperty(window, "isSecureContext");
    }
  });

  it("disables current location and explains the HTTPS requirement on LAN HTTP", async () => {
    Object.defineProperty(window, "isSecureContext", { configurable: true, value: false });
    const screen = await renderWithProviders(
      <LocationSection locked={false} value={undefined} onValueChange={vi.fn()} />,
    );

    await expect
      .element(screen.getByRole("button", { name: "Use current location" }))
      .toBeDisabled();
    await expect
      .element(screen.getByText("Current location requires HTTPS or localhost."))
      .toBeInTheDocument();
  });
});
