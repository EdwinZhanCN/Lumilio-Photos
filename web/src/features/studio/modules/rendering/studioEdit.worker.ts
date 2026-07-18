import {
  DEFAULT_STUDIO_ADJUSTMENTS,
  normalizeStudioAdjustments,
  type StudioEditAdjustments,
} from "../../model/editTypes";

type RenderEngine = "webgpu" | "webgl2" | "wasm-cpu" | "canvas-2d";

type LoadImageMessage = {
  type: "LOAD_IMAGE";
  payload: {
    requestId: number;
    blob: Blob;
    adjustments?: Partial<StudioEditAdjustments>;
    previewMaxSize?: number;
  };
};

type LoadImageDataMessage = {
  type: "LOAD_IMAGE_DATA";
  payload: {
    requestId: number;
    imageData: ImageData;
    originalWidth: number;
    originalHeight: number;
    adjustments?: Partial<StudioEditAdjustments>;
    previewMaxSize?: number;
  };
};

type RenderPreviewMessage = {
  type: "RENDER_PREVIEW";
  payload: {
    requestId: number;
    adjustments: Partial<StudioEditAdjustments>;
    previewMaxSize?: number;
  };
};

type ExportImageMessage = {
  type: "EXPORT_IMAGE";
  payload: {
    requestId: number;
    adjustments: Partial<StudioEditAdjustments>;
    format: "image/jpeg" | "image/png" | "image/webp";
    quality: number;
    maxSize?: number;
  };
};

type WorkerMessage =
  | LoadImageMessage
  | LoadImageDataMessage
  | RenderPreviewMessage
  | ExportImageMessage;

type ProcessedImage = {
  data: Uint8ClampedArray;
  width: number;
  height: number;
  engine: RenderEngine;
};

type WasmProcessResult = {
  success: boolean;
  data?: Uint8Array | number[];
  width: number;
  height: number;
  engine?: RenderEngine;
  error?: string;
};

type GpuNavigator = Navigator & {
  gpu?: {
    requestAdapter: () => Promise<unknown>;
  };
};

type StudioWasmModule = typeof import("../../../../wasm/studio/studio_wasm");

let sourceBitmap: ImageBitmap | null = null;
let sourceImageData: ImageData | null = null;
let sourceOriginalWidth = 0;
let sourceOriginalHeight = 0;
let wasmModulePromise: Promise<StudioWasmModule> | null = null;
let webGpuDevicePromise: Promise<unknown> | null = null;
let webGpuDisabled = false;
let webGl2Disabled = false;

const WEBGPU_SHADER = `
struct Adjustments {
  exposure: f32,
  contrast: f32,
  highlights: f32,
  shadows: f32,
  whites: f32,
  blacks: f32,
  temperature: f32,
  tint: f32,
  vibrance: f32,
  saturation: f32,
  clarity: f32,
  sharpness: f32,
  noise_reduction: f32,
  width: f32,
  height: f32,
  pad: f32,
}

@group(0) @binding(0) var input_texture: texture_2d<f32>;
@group(0) @binding(1) var output_texture: texture_storage_2d<rgba8unorm, write>;
@group(0) @binding(2) var<uniform> adjustments: Adjustments;

const LUMA_COEFF = vec3<f32>(0.2126, 0.7152, 0.0722);

fn get_luma(c: vec3<f32>) -> f32 {
  return dot(c, LUMA_COEFF);
}

fn srgb_to_linear(c: vec3<f32>) -> vec3<f32> {
  let cutoff = vec3<f32>(0.04045);
  let a = vec3<f32>(0.055);
  let higher = pow((c + a) / (1.0 + a), vec3<f32>(2.4));
  let lower = c / 12.92;
  return select(higher, lower, c <= cutoff);
}

fn linear_to_srgb(c: vec3<f32>) -> vec3<f32> {
  let c_clamped = clamp(c, vec3<f32>(0.0), vec3<f32>(1.0));
  let cutoff = vec3<f32>(0.0031308);
  let a = vec3<f32>(0.055);
  let higher = (1.0 + a) * pow(c_clamped, vec3<f32>(1.0 / 2.4)) - a;
  let lower = c_clamped * 12.92;
  return select(higher, lower, c_clamped <= cutoff);
}

fn get_shadow_mult(luma: f32, sh: f32, bl: f32) -> f32 {
  var mult = 1.0;
  let safe_luma = max(luma, 0.0001);

  if (bl != 0.0) {
    let limit = 0.05;
    if (safe_luma < limit) {
      let x = safe_luma / limit;
      let mask = (1.0 - x) * (1.0 - x);
      let factor = min(exp2(bl * 0.75), 3.9);
      mult *= mix(1.0, factor, mask);
    }
  }
  if (sh != 0.0) {
    let limit = 0.1;
    if (safe_luma < limit) {
      let x = safe_luma / limit;
      let mask = (1.0 - x) * (1.0 - x);
      let factor = min(exp2(sh * 1.5), 3.9);
      mult *= mix(1.0, factor, mask);
    }
  }
  return mult;
}

fn apply_tonal_adjustments(color: vec3<f32>, blurred_linear: vec3<f32>, con: f32, sh: f32, wh: f32, bl: f32) -> vec3<f32> {
  var rgb = color;
  var blurred = blurred_linear;

  if (wh != 0.0) {
    let white_level = 1.0 - wh * 0.25;
    let w_mult = 1.0 / max(white_level, 0.01);
    rgb *= w_mult;
    blurred *= w_mult;
  }

  let pixel_luma = get_luma(max(rgb, vec3<f32>(0.0)));
  let blurred_luma = get_luma(max(blurred, vec3<f32>(0.0)));
  let edge_diff = abs(pow(max(pixel_luma, 0.0001), 0.5) - pow(max(blurred_luma, 0.0001), 0.5));
  let halo_protection = smoothstep(0.05, 0.25, edge_diff);

  if (sh != 0.0 || bl != 0.0) {
    let spatial_mult = get_shadow_mult(max(blurred_luma, 0.0001), sh, bl);
    let pixel_mult = get_shadow_mult(max(pixel_luma, 0.0001), sh, bl);
    rgb *= mix(spatial_mult, pixel_mult, halo_protection);
  }

  if (con != 0.0) {
    let safe_rgb = max(rgb, vec3<f32>(0.0));
    let g = 2.2;
    let perceptual = pow(safe_rgb, vec3<f32>(1.0 / g));
    let clamped_perceptual = clamp(perceptual, vec3<f32>(0.0), vec3<f32>(1.0));
    let strength = pow(2.0, con * 1.25);
    let high_part = 1.0 - 0.5 * pow(2.0 * (1.0 - clamped_perceptual), vec3<f32>(strength));
    let low_part = 0.5 * pow(2.0 * clamped_perceptual, vec3<f32>(strength));
    let curved_perceptual = select(high_part, low_part, clamped_perceptual < vec3<f32>(0.5));
    let contrast_adjusted_rgb = pow(curved_perceptual, vec3<f32>(g));
    let mix_factor = smoothstep(vec3<f32>(1.0), vec3<f32>(1.01), safe_rgb);
    rgb = mix(contrast_adjusted_rgb, rgb, mix_factor);
  }
  return rgb;
}

fn apply_highlights_adjustment(color_in: vec3<f32>, highlights_adj: f32) -> vec3<f32> {
  if (highlights_adj == 0.0) { return color_in; }

  let pixel_luma = get_luma(max(color_in, vec3<f32>(0.0)));
  let highlight_mask = smoothstep(0.3, 0.95, tanh(max(pixel_luma, 0.0001) * 1.5));
  if (highlight_mask < 0.001) { return color_in; }

  var final_adjusted_color: vec3<f32>;
  if (highlights_adj < 0.0) {
    var new_luma: f32;
    if (pixel_luma <= 1.0) {
      let gamma = 1.0 - highlights_adj * 1.75;
      new_luma = pow(pixel_luma, gamma);
    } else {
      let luma_excess = pixel_luma - 1.0;
      let compression_strength = -highlights_adj * 6.0;
      let compressed_excess = luma_excess / (1.0 + luma_excess * compression_strength);
      new_luma = 1.0 + compressed_excess;
    }
    let tonally_adjusted_color = color_in * (new_luma / max(pixel_luma, 0.0001));
    let desaturation_amount = smoothstep(1.0, 10.0, pixel_luma);
    final_adjusted_color = mix(tonally_adjusted_color, vec3<f32>(new_luma), desaturation_amount);
  } else {
    final_adjusted_color = color_in * pow(2.0, highlights_adj * 1.75);
  }

  return mix(color_in, final_adjusted_color, highlight_mask);
}

fn apply_creative_color(color: vec3<f32>, sat: f32, vib: f32) -> vec3<f32> {
  var processed = color;
  let luma = get_luma(processed);

  if (sat != 0.0) {
    processed = mix(vec3<f32>(luma), processed, 1.0 + sat);
  }
  if (vib == 0.0) { return processed; }

  let c_max = max(processed.r, max(processed.g, processed.b));
  let c_min = min(processed.r, min(processed.g, processed.b));
  let delta = c_max - c_min;
  if (delta < 0.02) { return processed; }

  let current_sat = delta / max(c_max, 0.001);
  var amount: f32;
  if (vib > 0.0) {
    let sat_mask = 1.0 - smoothstep(0.4, 0.9, current_sat);
    amount = vib * sat_mask * 3.0;
  } else {
    let desat_mask = 1.0 - smoothstep(0.2, 0.8, current_sat);
    amount = vib * desat_mask;
  }
  return mix(vec3<f32>(luma), processed, 1.0 + amount);
}

fn apply_white_balance(color: vec3<f32>, temp: f32, tnt: f32) -> vec3<f32> {
  let temp_kelvin_mult = vec3<f32>(1.0 + temp * 0.2, 1.0 + temp * 0.05, 1.0 - temp * 0.2);
  let tint_mult = vec3<f32>(1.0 + tnt * 0.25, 1.0 - tnt * 0.25, 1.0 + tnt * 0.25);
  return color * temp_kelvin_mult * tint_mult;
}

fn sample_linear(coord: vec2<i32>) -> vec3<f32> {
  let dims = vec2<i32>(textureDimensions(input_texture));
  let safe_coord = clamp(coord, vec2<i32>(0), dims - vec2<i32>(1));
  return srgb_to_linear(textureLoad(input_texture, safe_coord, 0).rgb);
}

fn average_linear(coord: vec2<i32>) -> vec3<f32> {
  var sum = vec3<f32>(0.0);
  for (var y = -1; y <= 1; y = y + 1) {
    for (var x = -1; x <= 1; x = x + 1) {
      sum += sample_linear(coord + vec2<i32>(x, y));
    }
  }
  return sum / 9.0;
}

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
  let dims = textureDimensions(input_texture);
  if (id.x >= dims.x || id.y >= dims.y) { return; }

  let coord = vec2<i32>(id.xy);
  var color = sample_linear(coord);
  let blurred = average_linear(coord);

  let noise_amount = clamp(adjustments.noise_reduction / 100.0, 0.0, 1.0) * 0.75;
  color = mix(color, blurred, noise_amount);

  let sharpness = max(adjustments.sharpness / 100.0, 0.0) * 0.65;
  color = max(color + (color - blurred) * sharpness, vec3<f32>(0.0));

  color *= pow(2.0, adjustments.exposure);
  color = apply_highlights_adjustment(color, adjustments.highlights / 100.0);
  color = apply_tonal_adjustments(color, blurred, adjustments.contrast / 100.0, adjustments.shadows / 100.0, adjustments.whites / 100.0, adjustments.blacks / 100.0);
  color = apply_white_balance(color, adjustments.temperature / 100.0, adjustments.tint / 100.0);
  color = apply_creative_color(color, adjustments.saturation / 100.0, adjustments.vibrance / 100.0);

  let clarity = adjustments.clarity / 100.0;
  if (clarity != 0.0) {
    let lum = get_luma(color);
    color = mix(vec3<f32>(lum), color, 1.0 + clarity * 0.18);
  }

  textureStore(output_texture, vec2<i32>(id.xy), vec4<f32>(linear_to_srgb(color), textureLoad(input_texture, coord, 0).a));
}
`;

const WEBGL_VERTEX_SHADER = `#version 300 es
in vec2 a_position;
in vec2 a_uv;
out vec2 v_uv;

void main() {
  v_uv = a_uv;
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

const WEBGL_FRAGMENT_SHADER = `#version 300 es
precision highp float;
precision highp sampler2D;

uniform sampler2D u_image;
uniform vec2 u_textureSize;
uniform float u_exposure;
uniform float u_contrast;
uniform float u_highlights;
uniform float u_shadows;
uniform float u_whites;
uniform float u_blacks;
uniform float u_temperature;
uniform float u_tint;
uniform float u_vibrance;
uniform float u_saturation;
uniform float u_clarity;
uniform float u_sharpness;
uniform float u_noiseReduction;

in vec2 v_uv;
out vec4 outColor;

const vec3 LUMA_COEFF = vec3(0.2126, 0.7152, 0.0722);

float get_luma(vec3 c) {
  return dot(c, LUMA_COEFF);
}

vec3 srgb_to_linear(vec3 c) {
  vec3 cutoff = vec3(0.04045);
  vec3 higher = pow((c + vec3(0.055)) / vec3(1.055), vec3(2.4));
  vec3 lower = c / 12.92;
  return mix(higher, lower, lessThanEqual(c, cutoff));
}

vec3 linear_to_srgb(vec3 c) {
  vec3 c_clamped = clamp(c, vec3(0.0), vec3(1.0));
  vec3 higher = vec3(1.055) * pow(c_clamped, vec3(1.0 / 2.4)) - vec3(0.055);
  vec3 lower = c_clamped * 12.92;
  return mix(higher, lower, lessThanEqual(c_clamped, vec3(0.0031308)));
}

float get_shadow_mult(float luma, float sh, float bl) {
  float mult = 1.0;
  float safe_luma = max(luma, 0.0001);
  if (bl != 0.0) {
    float limit = 0.05;
    if (safe_luma < limit) {
      float x = safe_luma / limit;
      float mask = (1.0 - x) * (1.0 - x);
      float factor = min(exp2(bl * 0.75), 3.9);
      mult *= mix(1.0, factor, mask);
    }
  }
  if (sh != 0.0) {
    float limit = 0.1;
    if (safe_luma < limit) {
      float x = safe_luma / limit;
      float mask = (1.0 - x) * (1.0 - x);
      float factor = min(exp2(sh * 1.5), 3.9);
      mult *= mix(1.0, factor, mask);
    }
  }
  return mult;
}

vec3 apply_tonal_adjustments(vec3 color, vec3 blurred, float con, float sh, float wh, float bl) {
  vec3 rgb = color;
  if (wh != 0.0) {
    float w_mult = 1.0 / max(1.0 - wh * 0.25, 0.01);
    rgb *= w_mult;
    blurred *= w_mult;
  }

  float pixel_luma = get_luma(max(rgb, vec3(0.0)));
  float blurred_luma = get_luma(max(blurred, vec3(0.0)));
  float edge_diff = abs(pow(max(pixel_luma, 0.0001), 0.5) - pow(max(blurred_luma, 0.0001), 0.5));
  float halo_protection = smoothstep(0.05, 0.25, edge_diff);

  if (sh != 0.0 || bl != 0.0) {
    float spatial_mult = get_shadow_mult(max(blurred_luma, 0.0001), sh, bl);
    float pixel_mult = get_shadow_mult(max(pixel_luma, 0.0001), sh, bl);
    rgb *= mix(spatial_mult, pixel_mult, halo_protection);
  }

  if (con != 0.0) {
    vec3 safe_rgb = max(rgb, vec3(0.0));
    float g = 2.2;
    vec3 perceptual = pow(safe_rgb, vec3(1.0 / g));
    vec3 clamped_perceptual = clamp(perceptual, vec3(0.0), vec3(1.0));
    float strength = pow(2.0, con * 1.25);
    vec3 high_part = 1.0 - 0.5 * pow(2.0 * (1.0 - clamped_perceptual), vec3(strength));
    vec3 low_part = 0.5 * pow(2.0 * clamped_perceptual, vec3(strength));
    vec3 curved_perceptual = mix(high_part, low_part, lessThan(clamped_perceptual, vec3(0.5)));
    vec3 contrast_adjusted_rgb = pow(curved_perceptual, vec3(g));
    vec3 mix_factor = smoothstep(vec3(1.0), vec3(1.01), safe_rgb);
    rgb = mix(contrast_adjusted_rgb, rgb, mix_factor);
  }
  return rgb;
}

vec3 apply_highlights_adjustment(vec3 color_in, float highlights_adj) {
  if (highlights_adj == 0.0) { return color_in; }

  float pixel_luma = get_luma(max(color_in, vec3(0.0)));
  float highlight_mask = smoothstep(0.3, 0.95, tanh(max(pixel_luma, 0.0001) * 1.5));
  if (highlight_mask < 0.001) { return color_in; }

  vec3 final_adjusted_color;
  if (highlights_adj < 0.0) {
    float new_luma;
    if (pixel_luma <= 1.0) {
      new_luma = pow(pixel_luma, 1.0 - highlights_adj * 1.75);
    } else {
      float luma_excess = pixel_luma - 1.0;
      float compressed_excess = luma_excess / (1.0 + luma_excess * -highlights_adj * 6.0);
      new_luma = 1.0 + compressed_excess;
    }
    vec3 tonally_adjusted_color = color_in * (new_luma / max(pixel_luma, 0.0001));
    final_adjusted_color = mix(tonally_adjusted_color, vec3(new_luma), smoothstep(1.0, 10.0, pixel_luma));
  } else {
    final_adjusted_color = color_in * pow(2.0, highlights_adj * 1.75);
  }
  return mix(color_in, final_adjusted_color, highlight_mask);
}

vec3 apply_white_balance(vec3 color, float temp, float tnt) {
  vec3 temp_kelvin_mult = vec3(1.0 + temp * 0.2, 1.0 + temp * 0.05, 1.0 - temp * 0.2);
  vec3 tint_mult = vec3(1.0 + tnt * 0.25, 1.0 - tnt * 0.25, 1.0 + tnt * 0.25);
  return color * temp_kelvin_mult * tint_mult;
}

vec3 apply_creative_color(vec3 color, float sat, float vib) {
  vec3 processed = color;
  float lum = get_luma(processed);

  if (sat != 0.0) {
    processed = mix(vec3(lum), processed, 1.0 + sat);
  }
  if (vib == 0.0) { return processed; }

  float c_max = max(processed.r, max(processed.g, processed.b));
  float c_min = min(processed.r, min(processed.g, processed.b));
  float delta = c_max - c_min;
  if (delta < 0.02) { return processed; }

  float current_sat = delta / max(c_max, 0.001);
  float amount;
  if (vib > 0.0) {
    amount = vib * (1.0 - smoothstep(0.4, 0.9, current_sat)) * 3.0;
  } else {
    amount = vib * (1.0 - smoothstep(0.2, 0.8, current_sat));
  }
  return mix(vec3(lum), processed, 1.0 + amount);
}

vec3 sample_linear(vec2 uv) {
  return srgb_to_linear(texture(u_image, clamp(uv, vec2(0.0), vec2(1.0))).rgb);
}

vec3 average_linear(vec2 uv) {
  vec2 texel = 1.0 / u_textureSize;
  vec3 sum = vec3(0.0);
  for (int y = -1; y <= 1; y++) {
    for (int x = -1; x <= 1; x++) {
      sum += sample_linear(uv + vec2(float(x), float(y)) * texel);
    }
  }
  return sum / 9.0;
}

void main() {
  vec4 original = texture(u_image, v_uv);
  vec3 color = srgb_to_linear(original.rgb);
  vec3 blurred = average_linear(v_uv);

  color = mix(color, blurred, clamp(u_noiseReduction / 100.0, 0.0, 1.0) * 0.75);
  color = max(color + (color - blurred) * max(u_sharpness / 100.0, 0.0) * 0.65, vec3(0.0));
  color *= pow(2.0, u_exposure);
  color = apply_highlights_adjustment(color, u_highlights / 100.0);
  color = apply_tonal_adjustments(color, blurred, u_contrast / 100.0, u_shadows / 100.0, u_whites / 100.0, u_blacks / 100.0);
  color = apply_white_balance(color, u_temperature / 100.0, u_tint / 100.0);
  color = apply_creative_color(color, u_saturation / 100.0, u_vibrance / 100.0);

  float clarity = u_clarity / 100.0;
  if (clarity != 0.0) {
    float lum = get_luma(color);
    color = mix(vec3(lum), color, 1.0 + clarity * 0.18);
  }

  outColor = vec4(linear_to_srgb(color), original.a);
}
`;

function scaleForMaxSize(width: number, height: number, maxSize: number): number {
  const longest = Math.max(width, height);
  if (longest <= maxSize) return 1;
  return maxSize / longest;
}

function getRenderSize(
  width: number,
  height: number,
  adjustments: StudioEditAdjustments,
  maxSize: number,
): { width: number; height: number; angle: number; scale: number } {
  const normalizedRotation = ((adjustments.rotation % 360) + 360) % 360;
  const quarterTurn = normalizedRotation === 90 || normalizedRotation === 270;
  const rotatedWidth = quarterTurn ? height : width;
  const rotatedHeight = quarterTurn ? width : height;
  const scale = scaleForMaxSize(rotatedWidth, rotatedHeight, maxSize);

  return {
    width: Math.max(1, Math.round(rotatedWidth * scale)),
    height: Math.max(1, Math.round(rotatedHeight * scale)),
    angle: normalizedRotation,
    scale,
  };
}

function drawSourceImageData(
  source: ImageBitmap | ImageData,
  adjustments: StudioEditAdjustments,
  maxSize: number,
): ImageData {
  const { canvas, width, height } = drawSourceCanvas(source, adjustments, maxSize);
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  if (!ctx) {
    throw new Error("2D canvas is not available in Studio worker");
  }
  return ctx.getImageData(0, 0, width, height);
}

function drawSourceCanvas(
  source: ImageBitmap | ImageData,
  adjustments: StudioEditAdjustments,
  maxSize: number,
): { canvas: OffscreenCanvas; width: number; height: number } {
  const sourceWidth = source.width;
  const sourceHeight = source.height;
  const render = getRenderSize(sourceWidth, sourceHeight, adjustments, maxSize);
  const canvas = new OffscreenCanvas(render.width, render.height);
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("2D canvas is not available in Studio worker");
  }

  let drawable: ImageBitmap | OffscreenCanvas = source as ImageBitmap;
  if (source instanceof ImageData) {
    const sourceCanvas = new OffscreenCanvas(source.width, source.height);
    const sourceCtx = sourceCanvas.getContext("2d");
    if (!sourceCtx) {
      throw new Error("2D canvas is not available for Studio source image");
    }
    sourceCtx.putImageData(source, 0, 0);
    drawable = sourceCanvas;
  }

  ctx.clearRect(0, 0, render.width, render.height);
  ctx.save();
  ctx.translate(render.width / 2, render.height / 2);
  ctx.rotate((render.angle * Math.PI) / 180);
  ctx.scale(
    adjustments.flipHorizontal ? -render.scale : render.scale,
    adjustments.flipVertical ? -render.scale : render.scale,
  );
  ctx.drawImage(drawable, -sourceWidth / 2, -sourceHeight / 2);
  ctx.restore();

  return { canvas, width: render.width, height: render.height };
}

function packAdjustments(
  adjustments: StudioEditAdjustments,
  width: number,
  height: number,
): Float32Array {
  return new Float32Array([
    adjustments.exposure,
    adjustments.contrast,
    adjustments.highlights,
    adjustments.shadows,
    adjustments.whites,
    adjustments.blacks,
    adjustments.temperature,
    adjustments.tint,
    adjustments.vibrance,
    adjustments.saturation,
    adjustments.clarity,
    adjustments.sharpness,
    adjustments.noiseReduction,
    width,
    height,
    0,
  ]);
}

function hasPhotometricAdjustments(adjustments: StudioEditAdjustments): boolean {
  return (
    adjustments.exposure !== 0 ||
    adjustments.contrast !== 0 ||
    adjustments.highlights !== 0 ||
    adjustments.shadows !== 0 ||
    adjustments.whites !== 0 ||
    adjustments.blacks !== 0 ||
    adjustments.temperature !== 0 ||
    adjustments.tint !== 0 ||
    adjustments.vibrance !== 0 ||
    adjustments.saturation !== 0 ||
    adjustments.clarity !== 0 ||
    adjustments.sharpness !== 0 ||
    adjustments.noiseReduction !== 0
  );
}

async function getWebGpuDevice(): Promise<unknown> {
  if (!webGpuDevicePromise) {
    webGpuDevicePromise = (async () => {
      const gpu = (navigator as GpuNavigator).gpu;
      if (!gpu) throw new Error("WebGPU is unavailable");
      const adapter = await gpu.requestAdapter();
      if (!adapter || typeof adapter !== "object" || !("requestDevice" in adapter)) {
        throw new Error("WebGPU adapter is unavailable");
      }
      return (adapter as { requestDevice: () => Promise<unknown> }).requestDevice();
    })();
  }
  return webGpuDevicePromise;
}

async function processWithWebGpu(
  imageData: ImageData,
  adjustments: StudioEditAdjustments,
): Promise<ProcessedImage> {
  const device = (await getWebGpuDevice()) as {
    createTexture: (descriptor: unknown) => unknown;
    createBuffer: (descriptor: unknown) => unknown;
    createShaderModule: (descriptor: unknown) => unknown;
    createComputePipeline: (descriptor: unknown) => unknown;
    createBindGroup: (descriptor: unknown) => unknown;
    createCommandEncoder: () => unknown;
    queue: {
      writeTexture: (
        destination: unknown,
        data: Uint8ClampedArray | Float32Array,
        layout: unknown,
        size: unknown,
      ) => void;
      writeBuffer: (buffer: unknown, offset: number, data: Float32Array) => void;
      submit: (commands: unknown[]) => void;
    };
  };
  const gpuGlobal = globalThis as typeof globalThis & {
    GPUBufferUsage: { COPY_DST: number; MAP_READ: number; UNIFORM: number };
    GPUMapMode: { READ: number };
    GPUTextureUsage: {
      COPY_DST: number;
      COPY_SRC: number;
      STORAGE_BINDING: number;
      TEXTURE_BINDING: number;
    };
  };
  const width = imageData.width;
  const height = imageData.height;

  const inputTexture = device.createTexture({
    size: [width, height, 1],
    format: "rgba8unorm",
    usage: gpuGlobal.GPUTextureUsage.TEXTURE_BINDING | gpuGlobal.GPUTextureUsage.COPY_DST,
  }) as { createView: () => unknown };
  const outputTexture = device.createTexture({
    size: [width, height, 1],
    format: "rgba8unorm",
    usage: gpuGlobal.GPUTextureUsage.STORAGE_BINDING | gpuGlobal.GPUTextureUsage.COPY_SRC,
  }) as { createView: () => unknown };

  device.queue.writeTexture(
    { texture: inputTexture },
    imageData.data,
    { bytesPerRow: width * 4, rowsPerImage: height },
    { width, height, depthOrArrayLayers: 1 },
  );

  const adjustmentBuffer = device.createBuffer({
    size: 64,
    usage: gpuGlobal.GPUBufferUsage.UNIFORM | gpuGlobal.GPUBufferUsage.COPY_DST,
  });
  device.queue.writeBuffer(adjustmentBuffer, 0, packAdjustments(adjustments, width, height));

  const shader = device.createShaderModule({ code: WEBGPU_SHADER });
  const pipeline = device.createComputePipeline({
    layout: "auto",
    compute: {
      module: shader,
      entryPoint: "main",
    },
  }) as { getBindGroupLayout: (index: number) => unknown };

  const bindGroup = device.createBindGroup({
    layout: pipeline.getBindGroupLayout(0),
    entries: [
      { binding: 0, resource: inputTexture.createView() },
      { binding: 1, resource: outputTexture.createView() },
      { binding: 2, resource: { buffer: adjustmentBuffer } },
    ],
  });

  const bytesPerRow = Math.ceil((width * 4) / 256) * 256;
  const outputBuffer = device.createBuffer({
    size: bytesPerRow * height,
    usage: gpuGlobal.GPUBufferUsage.COPY_DST | gpuGlobal.GPUBufferUsage.MAP_READ,
  }) as {
    mapAsync: (mode: number) => Promise<void>;
    getMappedRange: () => ArrayBuffer;
    unmap: () => void;
  };

  const encoder = device.createCommandEncoder() as {
    beginComputePass: () => unknown;
    copyTextureToBuffer: (source: unknown, destination: unknown, size: unknown) => void;
    finish: () => unknown;
  };
  const pass = encoder.beginComputePass() as {
    setPipeline: (pipeline: unknown) => void;
    setBindGroup: (index: number, bindGroup: unknown) => void;
    dispatchWorkgroups: (x: number, y: number) => void;
    end: () => void;
  };
  pass.setPipeline(pipeline);
  pass.setBindGroup(0, bindGroup);
  pass.dispatchWorkgroups(Math.ceil(width / 8), Math.ceil(height / 8));
  pass.end();
  encoder.copyTextureToBuffer(
    { texture: outputTexture },
    { buffer: outputBuffer, bytesPerRow, rowsPerImage: height },
    { width, height, depthOrArrayLayers: 1 },
  );
  device.queue.submit([encoder.finish()]);

  await outputBuffer.mapAsync(gpuGlobal.GPUMapMode.READ);
  const mapped = new Uint8Array(outputBuffer.getMappedRange());
  const result = new Uint8ClampedArray(width * height * 4);
  for (let y = 0; y < height; y += 1) {
    const sourceStart = y * bytesPerRow;
    const targetStart = y * width * 4;
    result.set(mapped.subarray(sourceStart, sourceStart + width * 4), targetStart);
  }
  outputBuffer.unmap();

  return { data: result, width, height, engine: "webgpu" };
}

function compileShader(gl: WebGL2RenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error("Failed to create WebGL shader");
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const info = gl.getShaderInfoLog(shader) ?? "Unknown WebGL shader error";
    gl.deleteShader(shader);
    throw new Error(info);
  }
  return shader;
}

function assertNoWebGlError(gl: WebGL2RenderingContext, label: string): void {
  const error = gl.getError();
  if (error !== gl.NO_ERROR) {
    throw new Error(`${label} failed with WebGL error 0x${error.toString(16)}`);
  }
}

function processWithWebGl2(
  imageData: ImageData,
  adjustments: StudioEditAdjustments,
): ProcessedImage {
  const { width, height } = imageData;
  const canvas = new OffscreenCanvas(width, height);
  const gl = canvas.getContext("webgl2", {
    premultipliedAlpha: false,
    preserveDrawingBuffer: true,
  });
  if (!gl) {
    throw new Error("WebGL2 is unavailable");
  }

  const vertexShader = compileShader(gl, gl.VERTEX_SHADER, WEBGL_VERTEX_SHADER);
  const fragmentShader = compileShader(gl, gl.FRAGMENT_SHADER, WEBGL_FRAGMENT_SHADER);
  const program = gl.createProgram();
  if (!program) throw new Error("Failed to create WebGL program");
  const vao = gl.createVertexArray();
  const buffer = gl.createBuffer();
  const inputTexture = gl.createTexture();
  const outputTexture = gl.createTexture();
  const framebuffer = gl.createFramebuffer();

  try {
    gl.attachShader(program, vertexShader);
    gl.attachShader(program, fragmentShader);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      throw new Error(gl.getProgramInfoLog(program) ?? "Failed to link WebGL program");
    }
    gl.useProgram(program);

    if (!vao || !buffer || !inputTexture || !outputTexture || !framebuffer) {
      throw new Error("Failed to allocate WebGL2 resources");
    }

    gl.bindVertexArray(vao);
    const vertices = new Float32Array([-1, -1, 0, 1, 1, -1, 1, 1, -1, 1, 0, 0, 1, 1, 1, 0]);
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW);

    const positionLocation = gl.getAttribLocation(program, "a_position");
    const uvLocation = gl.getAttribLocation(program, "a_uv");
    gl.enableVertexAttribArray(positionLocation);
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 16, 0);
    gl.enableVertexAttribArray(uvLocation);
    gl.vertexAttribPointer(uvLocation, 2, gl.FLOAT, false, 16, 8);

    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, inputTexture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, 0);
    gl.texImage2D(
      gl.TEXTURE_2D,
      0,
      gl.RGBA,
      width,
      height,
      0,
      gl.RGBA,
      gl.UNSIGNED_BYTE,
      imageData.data,
    );
    assertNoWebGlError(gl, "WebGL input upload");

    gl.bindTexture(gl.TEXTURE_2D, outputTexture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);

    gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, outputTexture, 0);
    if (gl.checkFramebufferStatus(gl.FRAMEBUFFER) !== gl.FRAMEBUFFER_COMPLETE) {
      throw new Error("WebGL framebuffer is incomplete");
    }

    gl.uniform1i(gl.getUniformLocation(program, "u_image"), 0);
    gl.uniform2f(gl.getUniformLocation(program, "u_textureSize"), width, height);
    gl.uniform1f(gl.getUniformLocation(program, "u_exposure"), adjustments.exposure);
    gl.uniform1f(gl.getUniformLocation(program, "u_contrast"), adjustments.contrast);
    gl.uniform1f(gl.getUniformLocation(program, "u_highlights"), adjustments.highlights);
    gl.uniform1f(gl.getUniformLocation(program, "u_shadows"), adjustments.shadows);
    gl.uniform1f(gl.getUniformLocation(program, "u_whites"), adjustments.whites);
    gl.uniform1f(gl.getUniformLocation(program, "u_blacks"), adjustments.blacks);
    gl.uniform1f(gl.getUniformLocation(program, "u_temperature"), adjustments.temperature);
    gl.uniform1f(gl.getUniformLocation(program, "u_tint"), adjustments.tint);
    gl.uniform1f(gl.getUniformLocation(program, "u_vibrance"), adjustments.vibrance);
    gl.uniform1f(gl.getUniformLocation(program, "u_saturation"), adjustments.saturation);
    gl.uniform1f(gl.getUniformLocation(program, "u_clarity"), adjustments.clarity);
    gl.uniform1f(gl.getUniformLocation(program, "u_sharpness"), adjustments.sharpness);
    gl.uniform1f(gl.getUniformLocation(program, "u_noiseReduction"), adjustments.noiseReduction);

    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, inputTexture);
    gl.viewport(0, 0, width, height);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    assertNoWebGlError(gl, "WebGL draw");

    const pixels = new Uint8Array(width * height * 4);
    gl.readPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, pixels);
    assertNoWebGlError(gl, "WebGL readPixels");

    const flipped = new Uint8ClampedArray(width * height * 4);
    const rowBytes = width * 4;
    for (let y = 0; y < height; y += 1) {
      const sourceStart = (height - 1 - y) * rowBytes;
      flipped.set(pixels.subarray(sourceStart, sourceStart + rowBytes), y * rowBytes);
    }

    return { data: flipped, width, height, engine: "webgl2" };
  } finally {
    gl.deleteFramebuffer(framebuffer);
    gl.deleteTexture(outputTexture);
    gl.deleteTexture(inputTexture);
    gl.deleteBuffer(buffer);
    gl.deleteVertexArray(vao);
    gl.deleteShader(vertexShader);
    gl.deleteShader(fragmentShader);
    gl.deleteProgram(program);
  }
}

async function processWithWasmCpu(
  imageData: ImageData,
  adjustments: StudioEditAdjustments,
): Promise<ProcessedImage> {
  if (!wasmModulePromise) {
    wasmModulePromise = (async () => {
      const wasmModule = await import("../../../../wasm/studio/studio_wasm");
      await wasmModule.default(
        new URL("../../../../wasm/studio/studio_wasm_bg.wasm", import.meta.url),
      );
      return wasmModule;
    })();
  }
  const wasmModule = await wasmModulePromise;
  if (!wasmModule) {
    throw new Error("Studio WASM module was not initialized");
  }

  const result = wasmModule.process_rgba(
    new Uint8Array(imageData.data.buffer.slice(0)),
    imageData.width,
    imageData.height,
    adjustments,
  ) as WasmProcessResult;

  if (!result.success || !result.data) {
    throw new Error(result.error ?? "WASM CPU processing failed");
  }

  return {
    data: new Uint8ClampedArray(result.data),
    width: result.width,
    height: result.height,
    engine: "wasm-cpu",
  };
}

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, label: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      reject(new Error(`${label} timed out`));
    }, timeoutMs);

    promise
      .then((value) => {
        clearTimeout(timeoutId);
        resolve(value);
      })
      .catch((error: unknown) => {
        clearTimeout(timeoutId);
        reject(error);
      });
  });
}

async function processWithBestBackend(
  imageData: ImageData,
  adjustments: StudioEditAdjustments,
): Promise<ProcessedImage> {
  let lastGpuError: unknown;

  if (!webGpuDisabled) {
    try {
      return await withTimeout(processWithWebGpu(imageData, adjustments), 2500, "WebGPU");
    } catch (error) {
      webGpuDisabled = true;
      lastGpuError = error;
    }
  }

  if (!webGl2Disabled) {
    try {
      return processWithWebGl2(imageData, adjustments);
    } catch (error) {
      webGl2Disabled = true;
      lastGpuError = error;
    }
  }

  console.warn("Studio GPU render unavailable, using WASM CPU fallback", lastGpuError);
  return processWithWasmCpu(imageData, adjustments);
}

async function processedImageToBlob(
  processed: ProcessedImage,
  format: "image/jpeg" | "image/png" | "image/webp",
  quality: number,
): Promise<Blob> {
  const canvas = new OffscreenCanvas(processed.width, processed.height);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("2D canvas is not available for Studio export");
  const imageDataBytes = new Uint8ClampedArray(processed.data) as Uint8ClampedArray<ArrayBuffer>;
  ctx.putImageData(new ImageData(imageDataBytes, processed.width, processed.height), 0, 0);
  return canvas.convertToBlob({ type: format, quality });
}

async function canvasToBlob(
  canvas: OffscreenCanvas,
  format: "image/jpeg" | "image/png" | "image/webp",
  quality: number,
): Promise<Blob> {
  return canvas.convertToBlob({ type: format, quality });
}

async function renderImage(
  adjustmentsInput: Partial<StudioEditAdjustments> | undefined,
  maxSize: number,
  format: "image/jpeg" | "image/png" | "image/webp",
  quality: number,
): Promise<{ blob: Blob; width: number; height: number; engine: RenderEngine }> {
  const source = sourceImageData ?? sourceBitmap;
  if (!source) {
    throw new Error("No source image loaded");
  }

  const adjustments = normalizeStudioAdjustments(adjustmentsInput ?? DEFAULT_STUDIO_ADJUSTMENTS);
  if (!hasPhotometricAdjustments(adjustments)) {
    const rendered = drawSourceCanvas(source, adjustments, maxSize);
    const blob = await canvasToBlob(rendered.canvas, format, quality);
    return {
      blob,
      width: rendered.width,
      height: rendered.height,
      engine: "canvas-2d",
    };
  }

  const renderSourceImageData = drawSourceImageData(source, adjustments, maxSize);
  const processed = await processWithBestBackend(renderSourceImageData, adjustments);
  const blob = await processedImageToBlob(processed, format, quality);
  return {
    blob,
    width: processed.width,
    height: processed.height,
    engine: processed.engine,
  };
}

self.onmessage = async (event: MessageEvent<WorkerMessage>) => {
  try {
    const message = event.data;
    const requestId = message.payload.requestId;

    if (message.type === "LOAD_IMAGE") {
      if (sourceBitmap) {
        sourceBitmap.close();
      }
      sourceImageData = null;
      sourceBitmap = await withTimeout(
        createImageBitmap(message.payload.blob),
        5000,
        "Image decode",
      );
      sourceOriginalWidth = sourceBitmap.width;
      sourceOriginalHeight = sourceBitmap.height;
      const result = await renderImage(
        message.payload.adjustments,
        message.payload.previewMaxSize ?? 1800,
        "image/jpeg",
        0.9,
      );
      self.postMessage({
        type: "IMAGE_LOADED",
        payload: {
          requestId,
          blob: result.blob,
          width: result.width,
          height: result.height,
          engine: result.engine,
          originalWidth: sourceOriginalWidth,
          originalHeight: sourceOriginalHeight,
        },
      });
      return;
    }

    if (message.type === "LOAD_IMAGE_DATA") {
      if (sourceBitmap) {
        sourceBitmap.close();
        sourceBitmap = null;
      }
      sourceImageData = message.payload.imageData;
      sourceOriginalWidth = message.payload.originalWidth;
      sourceOriginalHeight = message.payload.originalHeight;
      const result = await renderImage(
        message.payload.adjustments,
        message.payload.previewMaxSize ?? 1800,
        "image/jpeg",
        0.9,
      );
      self.postMessage({
        type: "IMAGE_LOADED",
        payload: {
          requestId,
          blob: result.blob,
          width: result.width,
          height: result.height,
          engine: result.engine,
          originalWidth: sourceOriginalWidth,
          originalHeight: sourceOriginalHeight,
        },
      });
      return;
    }

    if (message.type === "RENDER_PREVIEW") {
      const result = await renderImage(
        message.payload.adjustments,
        message.payload.previewMaxSize ?? 1800,
        "image/jpeg",
        0.9,
      );
      self.postMessage({
        type: "PREVIEW_COMPLETE",
        payload: { requestId, ...result },
      });
      return;
    }

    if (message.type === "EXPORT_IMAGE") {
      const result = await renderImage(
        message.payload.adjustments,
        message.payload.maxSize ?? 8192,
        message.payload.format,
        message.payload.quality,
      );
      self.postMessage({
        type: "EXPORT_COMPLETE",
        payload: { requestId, ...result },
      });
    }
  } catch (error) {
    self.postMessage({
      type: "ERROR",
      payload: {
        requestId: event.data.payload.requestId,
        error: error instanceof Error ? error.message : "Studio edit worker failed",
      },
    });
  }
};

export {};
