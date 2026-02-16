import { hexToRgb, normalizeParams } from "./types";

type BorderWasmModule = {
  default: (input?: string | URL | Request | Response) => Promise<unknown>;
  add_colored_border: (
    image: Uint8Array,
    borderWidth: number,
    r: number,
    g: number,
    b: number,
    jpegQuality: number,
  ) => Uint8Array;
  create_frosted_border: (
    image: Uint8Array,
    blurSigma: number,
    brightnessAdjustment: number,
    cornerRadius: number,
    jpegQuality: number,
  ) => Uint8Array;
  add_vignette_border: (
    image: Uint8Array,
    strength: number,
    jpegQuality: number,
  ) => Uint8Array;
};

let wasmPromise: Promise<BorderWasmModule> | null = null;

async function getBorderWasm(): Promise<BorderWasmModule> {
  if (!wasmPromise) {
    wasmPromise = (async () => {
      const mod = (await import("./vendor/border_wasm.js")) as BorderWasmModule;
      await mod.default(new URL("./vendor/border_wasm_bg.wasm", import.meta.url));
      return mod;
    })();
  }
  return wasmPromise;
}

export async function run(
  ctx: { inputFile: File; signal: AbortSignal },
  rawParams: Record<string, unknown>,
  helpers?: { reportProgress?: (processed: number, total: number) => void },
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  helpers?.reportProgress?.(1, 4);

  const wasm = await getBorderWasm();
  const params = normalizeParams(rawParams);

  helpers?.reportProgress?.(2, 4);
  const inputBytes = new Uint8Array(await ctx.inputFile.arrayBuffer());
  if (ctx.signal.aborted) {
    throw new Error("Operation aborted");
  }

  let outputBytes: Uint8Array;

  if (params.mode === "COLORED") {
    const rgb = hexToRgb(params.color_hex);
    outputBytes = wasm.add_colored_border(
      inputBytes,
      params.border_width,
      rgb.r,
      rgb.g,
      rgb.b,
      params.jpeg_quality,
    );
  } else if (params.mode === "FROSTED") {
    outputBytes = wasm.create_frosted_border(
      inputBytes,
      params.blur_sigma,
      params.brightness_adjustment,
      params.corner_radius,
      params.jpeg_quality,
    );
  } else {
    outputBytes = wasm.add_vignette_border(
      inputBytes,
      params.strength,
      params.jpeg_quality,
    );
  }

  helpers?.reportProgress?.(4, 4);

  const base = ctx.inputFile.name.replace(/\.[^.]+$/, "");
  return {
    bytes: outputBytes,
    mimeType: "image/jpeg",
    fileName: `${base}-border.jpg`,
  };
}

export default {
  run,
};
