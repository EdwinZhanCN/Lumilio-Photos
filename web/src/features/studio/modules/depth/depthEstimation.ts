/**
 * Scene depth estimation with Depth Anything V2 (small) via transformers.js.
 *
 * MAIN THREAD. Produces a grayscale depth field (0 = far, 255 = near, matching
 * {@link buildDepthAlphaMask}'s convention) that the worker uses to occlude
 * layers behind nearer parts of the scene.
 *
 * The model and its onnxruntime backend are large, so transformers.js is
 * imported dynamically — it lands in its own chunk fetched only when someone
 * turns depth on, never in the entry bundle. Inference runs on WebGPU with the
 * q4f16 build; where WebGPU is unavailable the caller treats depth as an
 * unavailable capability and simply skips occlusion.
 */

const MODEL_ID = "onnx-community/depth-anything-v2-small-ONNX";

/** Minimal shapes of the transformers.js surfaces we touch. */
type RawImageLike = {
  data: Uint8Array | Uint8ClampedArray;
  width: number;
  height: number;
  channels: number;
};
type DepthOutput = { depth: RawImageLike };
type DepthPipeline = (input: unknown) => Promise<DepthOutput>;

let pipelinePromise: Promise<DepthPipeline> | null = null;

async function getDepthPipeline(): Promise<DepthPipeline> {
  if (!pipelinePromise) {
    pipelinePromise = (async () => {
      const transformers = await import("@huggingface/transformers");
      // Fully self-hosted: model from public/models, onnxruntime wasm from
      // public/ort — no HuggingFace CDN at runtime (vendored by
      // scripts/fetch-depth-model.mjs). Keeps the app local-first. In the browser
      // allowLocalModels defaults to false, so it must be turned on explicitly.
      transformers.env.allowLocalModels = true;
      transformers.env.allowRemoteModels = false;
      transformers.env.localModelPath = "/models/";
      const wasm = transformers.env.backends?.onnx?.wasm;
      // Absolute URL, not "/ort/": onnxruntime-web dynamically imports its loader
      // .mjs from wasmPaths, and Vite refuses to resolve a root-absolute /public
      // path as a module. A full origin URL is treated as external and fetched.
      if (wasm) wasm.wasmPaths = `${self.location.origin}/ort/`;

      const pipe = await transformers.pipeline("depth-estimation", MODEL_ID, {
        dtype: "q4f16",
        device: "webgpu",
      });
      return pipe as unknown as DepthPipeline;
    })();
  }
  return pipelinePromise;
}

export type DepthField = {
  /** Grayscale RGBA depth field (R=G=B=depth), ready to transfer to the worker. */
  bitmap: ImageBitmap;
  width: number;
  height: number;
};

/**
 * Estimate the depth field for `source` and return it as an ImageBitmap. Rejects
 * if the model or WebGPU is unavailable; callers degrade to no occlusion.
 */
const DEPTH_INPUT_MAX = 1024;

export async function estimateDepthField(source: Blob): Promise<DepthField> {
  const transformers = await import("@huggingface/transformers");
  const pipe = await getDepthPipeline();

  // The model interpolates its output back to the input size, so a full-res
  // source would yield a huge depth field. A ~1024px input keeps the field small
  // (occlusion is low-frequency and the mask is stretched to output anyway).
  const raw = await transformers.RawImage.fromBlob(source);
  const longest = Math.max(raw.width, raw.height);
  const image =
    longest > DEPTH_INPUT_MAX
      ? await raw.resize(
          Math.round((raw.width * DEPTH_INPUT_MAX) / longest),
          Math.round((raw.height * DEPTH_INPUT_MAX) / longest),
        )
      : raw;
  const { depth } = await pipe(image);

  const count = depth.width * depth.height;
  const rgba = new Uint8ClampedArray(count * 4);
  for (let i = 0; i < count; i += 1) {
    const value = depth.data[i * depth.channels];
    const p = i * 4;
    rgba[p] = value;
    rgba[p + 1] = value;
    rgba[p + 2] = value;
    rgba[p + 3] = 255;
  }

  const bitmap = await createImageBitmap(new ImageData(rgba, depth.width, depth.height));
  return { bitmap, width: depth.width, height: depth.height };
}

/** Free the cached pipeline (e.g. when leaving the editor). */
export function disposeDepthPipeline(): void {
  pipelinePromise = null;
}
