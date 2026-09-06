import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset } from "@/lib/assets/types";
import { downloadOriginalPhoto } from "../../api/exportPhoto";
import { AssetExportDialog } from "./AssetExportDialog";

const photo: Asset = {
  asset_id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  type: "PHOTO",
  original_filename: "travel.jpg",
  width: 3000,
  height: 2000,
  specific_metadata: {},
};
const cleanup: Array<() => void> = [];
afterEach(() => {
  cleanup.splice(0).forEach((fn) => fn());
});

function collectDownloads() {
  const downloads: Array<{ filename: string; bytes: Promise<number[]> }> = [];
  const listen = (event: MouseEvent) => {
    if (!(event.target instanceof HTMLAnchorElement) || !event.target.download) return;
    event.preventDefault();
    downloads.push({
      filename: event.target.download,
      bytes: fetch(event.target.href)
        .then((response) => response.arrayBuffer())
        .then((bytes) => Array.from(new Uint8Array(bytes))),
    });
  };
  document.addEventListener("click", listen);
  cleanup.push(() => document.removeEventListener("click", listen));
  return downloads;
}

describe("Fullscreen photo export", () => {
  it("retains settings on failure, then downloads exactly the successful response on retry", async () => {
    const requests: URL[] = [];
    worker.use(
      http.get("*/api/v1/assets/:id/export", ({ request }) => {
        requests.push(new URL(request.url));
        return requests.length === 1
          ? new HttpResponse(null, { status: 500 })
          : new HttpResponse(new Uint8Array([1, 2, 3]), {
              headers: { "Content-Type": "image/webp" },
            });
      }),
    );
    const downloads = collectDownloads();
    const close = vi.fn();
    const screen = await renderWithProviders(<AssetExportDialog asset={photo} onClose={close} />);
    await screen.getByLabelText(t("photoExport.format"), { exact: true }).selectOptions("webp");
    await screen.getByLabelText(t("photoExport.size"), { exact: true }).selectOptions("percent");
    await screen.getByLabelText(t("photoExport.filename"), { exact: true }).fill("selected.name");
    await screen.getByRole("button", { name: t("photoExport.confirm"), exact: true }).click();
    await expect.element(screen.getByRole("alert")).toBeVisible();
    expect(close).not.toHaveBeenCalled();
    expect(downloads).toHaveLength(0);
    await expect
      .element(screen.getByLabelText(t("photoExport.filename"), { exact: true }))
      .toHaveValue("selected.name");
    await screen.getByRole("button", { name: t("photoExport.confirm"), exact: true }).click();
    await expect.poll(() => close.mock.calls.length).toBe(1);
    expect(requests[1].search).toBe(requests[0].search);
    expect(Object.fromEntries(requests[1].searchParams)).toMatchObject({
      format: "webp",
      quality: "92",
      max_width: "1500",
      max_height: "1500",
      filename: "selected.name",
    });
    expect(downloads[0].filename).toBe("selected.name.webp");
    expect(await downloads[0].bytes).toEqual([1, 2, 3]);
  });
  it("downloads the original file without invoking conversion", async () => {
    const requests: string[] = [];
    worker.use(
      http.get("*/api/v1/assets/:id/original", ({ request }) => {
        requests.push(new URL(request.url).pathname);
        return new HttpResponse(new Uint8Array([255, 216, 7, 8, 255, 217]), {
          headers: { "Content-Type": "image/jpeg" },
        });
      }),
    );
    const blob = await downloadOriginalPhoto(photo);
    expect(Array.from(new Uint8Array(await blob.arrayBuffer()))).toEqual([
      255, 216, 7, 8, 255, 217,
    ]);
    expect(requests).toEqual([`/api/v1/assets/${photo.asset_id}/original`]);
  });
  it("does not deliver a fallback PNG as a JPEG", async () => {
    worker.use(
      http.get(
        "*/api/v1/assets/:id/export",
        () => new HttpResponse(new Uint8Array([1]), { headers: { "Content-Type": "image/png" } }),
      ),
    );
    const downloads = collectDownloads();
    const close = vi.fn();
    const screen = await renderWithProviders(<AssetExportDialog asset={photo} onClose={close} />);
    await screen.getByRole("button", { name: t("photoExport.confirm"), exact: true }).click();
    await expect.element(screen.getByRole("alert")).toBeVisible();
    expect(downloads).toHaveLength(0);
    expect(close).not.toHaveBeenCalled();
  });
});
