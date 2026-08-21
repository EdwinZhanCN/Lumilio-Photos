import { useState } from "react";
import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { t } from "@test/i18n";
import type { Asset } from "@/lib/http-commons";
import type { components } from "@/lib/http-commons/schema";
import FullScreenBasicInfo from "./FullScreenBasicInfo";

type AssetDetail = components["schemas"]["dto.AssetDetailDTO"];

const assetId = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa";
const photo: Asset = {
  asset_id: assetId,
  type: "PHOTO",
  original_filename: "receipt.jpg",
  mime_type: "image/jpeg",
  file_size: 1024,
  specific_metadata: {},
};
const secondPhoto: Asset = {
  ...photo,
  asset_id: "cccccccc-cccc-cccc-cccc-cccccccccccc",
  original_filename: "second.jpg",
};

function AssetSwitchHarness() {
  const [asset, setAsset] = useState(photo);
  return (
    <>
      <button type="button" onClick={() => setAsset(secondPhoto)}>
        switch test asset
      </button>
      <FullScreenBasicInfo asset={asset} />
    </>
  );
}

function serveTags() {
  const requests = vi.fn();
  worker.use(
    http.get(/\/api\/v1\/assets\/[^/]+\/tags$/, ({ params }) => {
      requests(params);
      return HttpResponse.json({ tags: [] });
    }),
  );
  return requests;
}

describe("FullScreenBasicInfo OCR Text Recognition", () => {
  it("loads only the active photo's OCR relation and renders provider order", async () => {
    serveTags();
    const seenQueries: URLSearchParams[] = [];
    const detail = {
      ...photo,
      ocr_result: {
        model_id: "fixture",
        total_count: 2,
        text_items: [
          { id: 1, text_content: "first provider line", confidence: 0.51 },
          { id: 2, text_content: "second provider line", confidence: 0.99 },
        ],
      },
    } satisfies AssetDetail;
    worker.use(
      http.get("*/api/v1/assets/:id", ({ request }) => {
        seenQueries.push(new URL(request.url).searchParams);
        return HttpResponse.json(detail);
      }),
    );

    const screen = await renderWithProviders(<FullScreenBasicInfo asset={photo} />);

    await expect
      .element(screen.getByRole("heading", { name: t("assets.ocr.title") }))
      .toBeVisible();
    await expect.element(screen.getByText("first provider line", { exact: true })).toBeVisible();
    await expect.element(screen.getByText("second provider line", { exact: true })).toBeVisible();
    expect(seenQueries).toHaveLength(1);
    expect(Object.fromEntries(seenQueries[0]!)).toEqual({
      include_albums: "false",
      include_faces: "false",
      include_ocr: "true",
      include_species: "false",
      include_tags: "false",
      include_thumbnails: "false",
    });
  });

  it("renders an unavailable state when no OCR result is stored", async () => {
    serveTags();
    worker.use(
      http.get("*/api/v1/assets/:id", () => HttpResponse.json({ ...photo } satisfies AssetDetail)),
    );

    const missingScreen = await renderWithProviders(<FullScreenBasicInfo asset={photo} />);
    await expect.element(missingScreen.getByText(t("assets.ocr.unavailable"))).toBeVisible();
  });

  it("distinguishes a stored zero-region OCR result", async () => {
    serveTags();
    worker.use(
      http.get("*/api/v1/assets/:id", () =>
        HttpResponse.json({
          ...photo,
          ocr_result: { model_id: "fixture", total_count: 0, text_items: [] },
        } satisfies AssetDetail),
      ),
    );

    const emptyScreen = await renderWithProviders(<FullScreenBasicInfo asset={photo} />);
    await expect.element(emptyScreen.getByText(t("assets.ocr.empty"))).toBeVisible();
  });

  it("offers retry after a relation request fails", async () => {
    serveTags();
    let requests = 0;
    worker.use(
      http.get("*/api/v1/assets/:id", () => {
        requests += 1;
        if (requests <= 2) {
          return HttpResponse.json({ message: "fixture failure" }, { status: 500 });
        }
        return HttpResponse.json({
          ...photo,
          ocr_result: {
            model_id: "fixture",
            total_count: 1,
            text_items: [{ id: 1, text_content: "loaded after retry", confidence: 0.8 }],
          },
        } satisfies AssetDetail);
      }),
    );

    const screen = await renderWithProviders(<FullScreenBasicInfo asset={photo} />);

    await expect.element(screen.getByRole("alert")).toBeVisible();
    await screen.getByRole("button", { name: t("common.retry") }).click();
    await expect.element(screen.getByText("loaded after retry", { exact: true })).toBeVisible();
    expect(requests).toBe(3);
  });

  it("copies recognized text in provider order", async () => {
    serveTags();
    const clipboardWriteText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: clipboardWriteText },
    });
    worker.use(
      http.get("*/api/v1/assets/:id", () =>
        HttpResponse.json({
          ...photo,
          ocr_result: {
            model_id: "fixture",
            total_count: 2,
            text_items: [
              { id: 1, text_content: "first line", confidence: 0.51 },
              { id: 2, text_content: "second line", confidence: 0.99 },
            ],
          },
        } satisfies AssetDetail),
      ),
    );

    const screen = await renderWithProviders(<FullScreenBasicInfo asset={photo} />);

    await screen.getByRole("button", { name: t("common.copy") }).click();
    await vi.waitFor(() => {
      expect(clipboardWriteText).toHaveBeenCalledWith("first line\nsecond line");
    });
  });

  it("rekeys OCR state when the active physical photo changes", async () => {
    serveTags();
    const requestedIds: string[] = [];
    worker.use(
      http.get("*/api/v1/assets/:id", ({ params }) => {
        const id = String(params.id);
        requestedIds.push(id);
        const active = id === secondPhoto.asset_id ? secondPhoto : photo;
        const line = id === secondPhoto.asset_id ? "second asset text" : "first asset text";
        return HttpResponse.json({
          ...active,
          ocr_result: {
            model_id: "fixture",
            total_count: 1,
            text_items: [{ id: 1, text_content: line, confidence: 0.8 }],
          },
        } satisfies AssetDetail);
      }),
    );

    const screen = await renderWithProviders(<AssetSwitchHarness />);
    await expect.element(screen.getByText("first asset text", { exact: true })).toBeVisible();

    await screen.getByRole("button", { name: "switch test asset" }).click();

    await expect.element(screen.getByText("second asset text", { exact: true })).toBeVisible();
    await expect
      .element(screen.getByText("first asset text", { exact: true }))
      .not.toBeInTheDocument();
    expect(requestedIds).toEqual([photo.asset_id, secondPhoto.asset_id]);
  });

  it("does not request OCR relations for non-photo assets", async () => {
    const tagRequest = serveTags();
    const detailRequest = vi.fn();
    worker.use(
      http.get("*/api/v1/assets/:id", () => {
        detailRequest();
        return HttpResponse.json({ ...photo } satisfies AssetDetail);
      }),
    );
    const video = {
      ...photo,
      asset_id: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      type: "VIDEO",
      original_filename: "clip.mp4",
      mime_type: "video/mp4",
    } satisfies Asset;

    const screen = await renderWithProviders(<FullScreenBasicInfo asset={video} />);

    await expect.element(screen.getByText("clip.mp4", { exact: true })).toBeVisible();
    await vi.waitFor(() => expect(tagRequest).toHaveBeenCalled());
    expect(detailRequest).not.toHaveBeenCalled();
  });
});
