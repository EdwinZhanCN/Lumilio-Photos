import { describe, it, expect } from "vite-plus/test";
import { DEFAULT_STUDIO_ADJUSTMENTS } from "../../model/editTypes";

// Real Chromium, real Worker, real WebGL2 + OffscreenCanvas. This is the whole
// preview/export pipeline over its actual message protocol: INIT_CANVAS hands
// over a canvas, LOAD_IMAGE develops, EXPORT_IMAGE encodes. The transferred
// visible canvas cannot be read back on the main thread, so correctness
// (including end-to-end orientation) is asserted through the exported blob.

function webgl2Available(): boolean {
  try {
    return Boolean(new OffscreenCanvas(1, 1).getContext("webgl2"));
  } catch {
    return false;
  }
}

let requestId = 0;

function call<T>(
  worker: Worker,
  type: string,
  payload: Record<string, unknown>,
  successType: string,
  transfer: Transferable[] = [],
): Promise<T> {
  const id = ++requestId;
  return new Promise<T>((resolve, reject) => {
    const timeout = setTimeout(() => {
      worker.removeEventListener("message", onMessage);
      reject(new Error(`${type} timed out`));
    }, 20000);
    const onMessage = (event: MessageEvent) => {
      const { type: t, payload: p } = event.data || {};
      if (!p || p.requestId !== id) return;
      if (t === successType) {
        clearTimeout(timeout);
        worker.removeEventListener("message", onMessage);
        resolve(p as T);
      } else if (t === "ERROR") {
        clearTimeout(timeout);
        worker.removeEventListener("message", onMessage);
        reject(new Error(p.error));
      }
    };
    worker.addEventListener("message", onMessage);
    worker.postMessage({ type, payload: { requestId: id, ...payload } }, transfer);
  });
}

/** Top-half red, bottom-half blue PNG so orientation is observable end to end. */
async function sourceBlob(): Promise<Blob> {
  const canvas = new OffscreenCanvas(64, 64);
  const ctx = canvas.getContext("2d")!;
  ctx.fillStyle = "rgb(200,0,0)";
  ctx.fillRect(0, 0, 64, 32);
  ctx.fillStyle = "rgb(0,0,200)";
  ctx.fillRect(0, 32, 64, 32);
  return canvas.convertToBlob({ type: "image/png" });
}

function createWorker(): Worker {
  return new Worker(new URL("./studioEdit.worker.ts", import.meta.url), { type: "module" });
}

describe.skipIf(!webgl2Available())("studioEdit worker", () => {
  it("loads a source and exports it upright over the real protocol", async () => {
    const worker = createWorker();
    try {
      const off = new OffscreenCanvas(8, 8);
      await call(worker, "INIT_CANVAS", { canvas: off }, "CANVAS_READY", [off]);

      const loaded = await call<{
        originalWidth: number;
        originalHeight: number;
        outWidth: number;
        outHeight: number;
        snapshot: ImageBitmap;
      }>(
        worker,
        "LOAD_IMAGE",
        { blob: await sourceBlob(), previewMaxSize: 1800, snapshotMaxSize: 256 },
        "IMAGE_LOADED",
      );
      expect([loaded.originalWidth, loaded.originalHeight]).toEqual([64, 64]);
      expect([loaded.outWidth, loaded.outHeight]).toEqual([64, 64]);
      expect(loaded.snapshot).toBeInstanceOf(ImageBitmap);
      loaded.snapshot.close();

      const exported = await call<{ blob: Blob; width: number; height: number }>(
        worker,
        "EXPORT_IMAGE",
        { adjustments: DEFAULT_STUDIO_ADJUSTMENTS, format: "image/png", quality: 1, sizeMode: { kind: "original" } },
        "EXPORT_COMPLETE",
      );
      expect([exported.width, exported.height]).toEqual([64, 64]);

      const bitmap = await createImageBitmap(exported.blob);
      const probe = new OffscreenCanvas(64, 64);
      const ctx = probe.getContext("2d")!;
      ctx.drawImage(bitmap, 0, 0);
      const top = ctx.getImageData(32, 10, 1, 1).data;
      const bottom = ctx.getImageData(32, 54, 1, 1).data;
      expect(Math.abs(top[0] - 200)).toBeLessThan(24); // top stays red
      expect(Math.abs(bottom[2] - 200)).toBeLessThan(24); // bottom stays blue
      bitmap.close();
    } finally {
      worker.terminate();
    }
  });

  it("applies a 90° rotation to the exported result", async () => {
    const worker = createWorker();
    try {
      const off = new OffscreenCanvas(8, 8);
      await call(worker, "INIT_CANVAS", { canvas: off }, "CANVAS_READY", [off]);
      await call(
        worker,
        "LOAD_IMAGE",
        { blob: await sourceBlob(), previewMaxSize: 1800, snapshotMaxSize: 256 },
        "IMAGE_LOADED",
      ).then((r) => (r as { snapshot: ImageBitmap }).snapshot.close());

      const exported = await call<{ blob: Blob; width: number; height: number }>(
        worker,
        "EXPORT_IMAGE",
        {
          adjustments: { ...DEFAULT_STUDIO_ADJUSTMENTS, rotation: 90 },
          format: "image/png",
          quality: 1,
          sizeMode: { kind: "original" },
        },
        "EXPORT_COMPLETE",
      );
      // A square stays 64×64, but the red top band rotates to a side band.
      const bitmap = await createImageBitmap(exported.blob);
      const probe = new OffscreenCanvas(64, 64);
      const ctx = probe.getContext("2d")!;
      ctx.drawImage(bitmap, 0, 0);
      // 90° clockwise: the top red band moves to the right edge.
      const right = ctx.getImageData(54, 32, 1, 1).data;
      const left = ctx.getImageData(10, 32, 1, 1).data;
      expect(right[0]).toBeGreaterThan(right[2]); // right side reddish
      expect(left[2]).toBeGreaterThan(left[0]); // left side bluish
      bitmap.close();
    } finally {
      worker.terminate();
    }
  });
});
