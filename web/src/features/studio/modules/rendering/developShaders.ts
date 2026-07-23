/**
 * Develop shader sources, shared by the render backends.
 *
 * The GLSL pair drives the WebGL2 backend used today; the WGSL compute shader is
 * kept for the WebGPU backend (Phase 1.5). All three implement the same
 * photometric algorithm ported from rapidraw — only the dispatch differs, so the
 * sources are colocated to stay in lockstep.
 *
 * WORKER-SAFE: plain strings, no DOM.
 */

export const WEBGPU_SHADER = `
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

export const WEBGL_VERTEX_SHADER = `#version 300 es
in vec2 a_position;
in vec2 a_uv;
out vec2 v_uv;

void main() {
  v_uv = a_uv;
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

export const WEBGL_FRAGMENT_SHADER = `#version 300 es
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
