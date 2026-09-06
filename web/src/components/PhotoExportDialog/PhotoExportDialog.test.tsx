import { useState } from "react";
import { describe, expect, it, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import {
  defaultPhotoExportSettings,
  type PhotoExportArtifact,
  type PhotoExportSettings,
} from "@/lib/photo-export/model";
import { PhotoExportDialog } from "./PhotoExportDialog";
import "@/styles/App.css";

function Harness({
  onExport,
  onClose = () => {},
}: {
  onExport: (settings: PhotoExportSettings, signal: AbortSignal) => Promise<PhotoExportArtifact>;
  onClose?: () => void;
}) {
  const [settings, setSettings] = useState(() => defaultPhotoExportSettings("travel.jpg"));
  return (
    <PhotoExportDialog
      open
      settings={settings}
      onChange={setSettings}
      formats={["jpeg", "png", "webp"]}
      sourceWidth={960}
      sourceHeight={1360}
      sourceLabel={t("photoExport.studioSource")}
      metadataNote={t("photoExport.studioMetadata")}
      onExport={onExport}
      onClose={onClose}
      onNotice={() => {}}
    />
  );
}

describe("shared photo export dialog", () => {
  it("locks settings and duplicate submission until the current export settles", async () => {
    let rejectExport: (reason: Error) => void = () => {};
    const pending = new Promise<PhotoExportArtifact>((_resolve, reject) => {
      rejectExport = reject;
    });
    const exportImage = vi.fn(() => pending);
    const screen = await renderWithProviders(<Harness onExport={exportImage} />);
    await screen.getByRole("button", { name: t("photoExport.confirm"), exact: true }).click();
    await expect
      .element(screen.getByRole("button", { name: t("photoExport.exporting"), exact: true }))
      .toBeDisabled();
    await expect
      .element(screen.getByLabelText(t("photoExport.filename"), { exact: true }))
      .toBeDisabled();
    expect(exportImage).toHaveBeenCalledTimes(1);
    rejectExport(new Error("fixture failure"));
    await expect.element(screen.getByRole("alert")).toBeVisible();
    await expect
      .element(screen.getByLabelText(t("photoExport.filename"), { exact: true }))
      .toBeEnabled();
  });
  it("offers only supported formats and Escape closes without starting an export", async () => {
    const exportImage = vi.fn();
    const close = vi.fn();
    const screen = await renderWithProviders(<Harness onExport={exportImage} onClose={close} />);
    const select = screen
      .getByRole("combobox", { name: t("photoExport.format"), exact: true })
      .element() as HTMLSelectElement;
    expect(Array.from(select.options, (option) => option.value)).toEqual(["jpeg", "png", "webp"]);
    const dialog = screen.getByRole("dialog").element();
    dialog.dispatchEvent(new Event("cancel", { cancelable: true }));
    expect(close).toHaveBeenCalledOnce();
    expect(exportImage).not.toHaveBeenCalled();
  });
  it("aborts delivery when its owner unmounts during an export", async () => {
    let finish: (artifact: PhotoExportArtifact) => void = () => {};
    const pending = new Promise<PhotoExportArtifact>((resolve) => {
      finish = resolve;
    });
    const signals: AbortSignal[] = [];
    const downloads: string[] = [];
    const listen = (event: MouseEvent) => {
      if (event.target instanceof HTMLAnchorElement && event.target.download) {
        event.preventDefault();
        downloads.push(event.target.download);
      }
    };
    document.addEventListener("click", listen);
    try {
      const screen = await renderWithProviders(
        <Harness
          onExport={(_settings, signal) => {
            signals.push(signal);
            return pending;
          }}
        />,
      );
      await screen.getByRole("button", { name: t("photoExport.confirm"), exact: true }).click();
      await screen.unmount();
      expect(signals[0].aborted).toBe(true);
      finish({ blob: new Blob(["fixture"], { type: "image/jpeg" }) });
      await pending;
      expect(downloads).toEqual([]);
    } finally {
      document.removeEventListener("click", listen);
    }
  });
  it("fits its controls at mobile and desktop widths", async () => {
    const screen = await renderWithProviders(<Harness onExport={vi.fn()} />);
    for (const width of [390, 1280]) {
      await page.viewport(width, 900);
      const dialog = screen.getByRole("dialog").element();
      const panel = dialog.querySelector(".modal-box")!;
      expect(panel.scrollWidth).toBeLessThanOrEqual(panel.clientWidth + 1);
      await expect
        .element(screen.getByRole("button", { name: t("photoExport.confirm"), exact: true }))
        .toBeVisible();
    }
  });
});
