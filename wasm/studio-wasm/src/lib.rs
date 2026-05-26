use serde::{Deserialize, Serialize};
use wasm_bindgen::prelude::*;

const LUMA_R: f32 = 0.2126;
const LUMA_G: f32 = 0.7152;
const LUMA_B: f32 = 0.0722;

#[derive(Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct StudioAdjustments {
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
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProcessResult {
    success: bool,
    data: Vec<u8>,
    width: u32,
    height: u32,
    engine: &'static str,
    error: Option<String>,
}

#[wasm_bindgen(start)]
pub fn start() {
    #[cfg(feature = "console_error_panic_hook")]
    console_error_panic_hook::set_once();
}

#[wasm_bindgen]
pub fn process_rgba(
    rgba: &[u8],
    width: u32,
    height: u32,
    adjustments_js: JsValue,
) -> Result<JsValue, JsValue> {
    let adjustments: StudioAdjustments =
        serde_wasm_bindgen::from_value(adjustments_js).map_err(|err| {
            JsValue::from_str(&format!("Invalid Studio adjustments: {err}"))
        })?;

    let expected_len = width as usize * height as usize * 4;
    if rgba.len() != expected_len {
        let result = ProcessResult {
            success: false,
            data: Vec::new(),
            width,
            height,
            engine: "wasm-cpu",
            error: Some(format!(
                "Invalid RGBA buffer length: expected {expected_len}, got {}",
                rgba.len()
            )),
        };
        return serde_wasm_bindgen::to_value(&result).map_err(|err| JsValue::from_str(&err.to_string()));
    }

    let processed = process_pixels(rgba, width, height, &adjustments);
    let result = ProcessResult {
        success: true,
        data: processed,
        width,
        height,
        engine: "wasm-cpu",
        error: None,
    };
    serde_wasm_bindgen::to_value(&result).map_err(|err| JsValue::from_str(&err.to_string()))
}

fn process_pixels(
    rgba: &[u8],
    width: u32,
    height: u32,
    adjustments: &StudioAdjustments,
) -> Vec<u8> {
    if adjustments.is_neutral() {
        return rgba.to_vec();
    }

    let mut out = vec![0; rgba.len()];
    let width_i = width as i32;
    let height_i = height as i32;
    let needs_blur = adjustments.needs_neighbor_average();

    for y in 0..height_i {
        for x in 0..width_i {
            let idx = ((y as u32 * width + x as u32) * 4) as usize;
            let mut color = [
                srgb_to_linear(rgba[idx] as f32 / 255.0),
                srgb_to_linear(rgba[idx + 1] as f32 / 255.0),
                srgb_to_linear(rgba[idx + 2] as f32 / 255.0),
            ];

            let blurred = if needs_blur {
                neighbor_average_linear(rgba, width, height, x, y)
            } else {
                color
            };
            color = apply_noise_reduction(color, blurred, adjustments.noise_reduction / 100.0);
            color = apply_sharpness(color, blurred, adjustments.sharpness / 100.0);
            color = apply_linear_exposure(color, adjustments.exposure);
            color = apply_filmic_exposure(color, adjustments.brightness());
            color = apply_highlights_adjustment(color, adjustments.highlights / 100.0);
            color = apply_tonal_adjustments(
                color,
                blurred,
                adjustments.contrast / 100.0,
                adjustments.shadows / 100.0,
                adjustments.whites / 100.0,
                adjustments.blacks / 100.0,
            );
            color = apply_white_balance(
                color,
                adjustments.temperature / 100.0,
                adjustments.tint / 100.0,
            );
            color = apply_creative_color(
                color,
                adjustments.saturation / 100.0,
                adjustments.vibrance / 100.0,
            );
            color = apply_clarity(color, adjustments.clarity / 100.0);

            out[idx] = (linear_to_srgb(color[0]).clamp(0.0, 1.0) * 255.0).round() as u8;
            out[idx + 1] = (linear_to_srgb(color[1]).clamp(0.0, 1.0) * 255.0).round() as u8;
            out[idx + 2] = (linear_to_srgb(color[2]).clamp(0.0, 1.0) * 255.0).round() as u8;
            out[idx + 3] = rgba[idx + 3];
        }
    }

    out
}

impl StudioAdjustments {
    fn brightness(&self) -> f32 {
        0.0
    }

    fn is_neutral(&self) -> bool {
        self.exposure == 0.0
            && self.contrast == 0.0
            && self.highlights == 0.0
            && self.shadows == 0.0
            && self.whites == 0.0
            && self.blacks == 0.0
            && self.temperature == 0.0
            && self.tint == 0.0
            && self.vibrance == 0.0
            && self.saturation == 0.0
            && self.clarity == 0.0
            && self.sharpness == 0.0
            && self.noise_reduction == 0.0
    }

    fn needs_neighbor_average(&self) -> bool {
        self.shadows != 0.0
            || self.blacks != 0.0
            || self.whites != 0.0
            || self.sharpness != 0.0
            || self.noise_reduction != 0.0
    }
}

fn sample_linear(rgba: &[u8], width: u32, height: u32, x: i32, y: i32) -> [f32; 3] {
    let xx = x.clamp(0, width as i32 - 1) as u32;
    let yy = y.clamp(0, height as i32 - 1) as u32;
    let idx = ((yy * width + xx) * 4) as usize;
    [
        srgb_to_linear(rgba[idx] as f32 / 255.0),
        srgb_to_linear(rgba[idx + 1] as f32 / 255.0),
        srgb_to_linear(rgba[idx + 2] as f32 / 255.0),
    ]
}

fn neighbor_average_linear(rgba: &[u8], width: u32, height: u32, x: i32, y: i32) -> [f32; 3] {
    let offsets = [
        (0, 0),
        (-1, 0),
        (1, 0),
        (0, -1),
        (0, 1),
        (-1, -1),
        (1, -1),
        (-1, 1),
        (1, 1),
    ];
    let mut sum = [0.0; 3];
    for (dx, dy) in offsets {
        let c = sample_linear(rgba, width, height, x + dx, y + dy);
        sum[0] += c[0];
        sum[1] += c[1];
        sum[2] += c[2];
    }
    [sum[0] / 9.0, sum[1] / 9.0, sum[2] / 9.0]
}

fn luma(c: [f32; 3]) -> f32 {
    c[0] * LUMA_R + c[1] * LUMA_G + c[2] * LUMA_B
}

fn srgb_to_linear(c: f32) -> f32 {
    if c <= 0.04045 {
        c / 12.92
    } else {
        ((c + 0.055) / 1.055).powf(2.4)
    }
}

fn linear_to_srgb(c: f32) -> f32 {
    let c = c.clamp(0.0, 1.0);
    if c <= 0.0031308 {
        c * 12.92
    } else {
        1.055 * c.powf(1.0 / 2.4) - 0.055
    }
}

fn mix(a: f32, b: f32, t: f32) -> f32 {
    a * (1.0 - t) + b * t
}

fn mix3(a: [f32; 3], b: [f32; 3], t: f32) -> [f32; 3] {
    [mix(a[0], b[0], t), mix(a[1], b[1], t), mix(a[2], b[2], t)]
}

fn smoothstep(edge0: f32, edge1: f32, x: f32) -> f32 {
    let t = ((x - edge0) / (edge1 - edge0)).clamp(0.0, 1.0);
    t * t * (3.0 - 2.0 * t)
}

fn apply_linear_exposure(color: [f32; 3], exposure_adj: f32) -> [f32; 3] {
    if exposure_adj == 0.0 {
        return color;
    }
    let factor = 2.0_f32.powf(exposure_adj);
    [color[0] * factor, color[1] * factor, color[2] * factor]
}

// Ported from RapidRAW's shader.wgsl filmic brightness curve.
fn apply_filmic_exposure(color: [f32; 3], brightness_adj: f32) -> [f32; 3] {
    if brightness_adj == 0.0 {
        return color;
    }
    const RATIONAL_CURVE_MIX: f32 = 0.95;
    const MIDTONE_STRENGTH: f32 = 1.2;
    const TOP_ANCHOR: f32 = 1.06;

    let original_luma = luma(color);
    if original_luma.abs() < 0.00001 {
        return color;
    }
    let direct_adj = brightness_adj * (1.0 - RATIONAL_CURVE_MIX);
    let rational_adj = brightness_adj * RATIONAL_CURVE_MIX;
    let scale = 2.0_f32.powf(direct_adj);
    let k = 2.0_f32.powf(-rational_adj * MIDTONE_STRENGTH);
    let luma_abs = original_luma.abs();
    let luma_floor = (luma_abs / TOP_ANCHOR).floor() * TOP_ANCHOR;
    let luma_norm = (luma_abs - luma_floor) / TOP_ANCHOR;
    let shaped_norm = luma_norm / (luma_norm + (1.0 - luma_norm) * k);
    let shaped_luma_abs = luma_floor + shaped_norm * TOP_ANCHOR;
    let new_luma = original_luma.signum() * shaped_luma_abs * scale;
    let total_luma_scale = new_luma / original_luma;
    let luma_weight = new_luma.clamp(0.0, 2.0) * 0.5;
    let dynamic_exp = mix(0.95, 0.65, luma_weight);
    let base_chroma_scale = total_luma_scale.powf(dynamic_exp);
    let highlight_rolloff = 1.0 / (1.0 + (new_luma - 0.9).max(0.0) * 2.0);
    let chroma_scale = base_chroma_scale * highlight_rolloff;
    [
        new_luma + (color[0] - original_luma) * chroma_scale,
        new_luma + (color[1] - original_luma) * chroma_scale,
        new_luma + (color[2] - original_luma) * chroma_scale,
    ]
}

fn shadow_mult(lum: f32, shadows: f32, blacks: f32) -> f32 {
    let safe_luma = lum.max(0.0001);
    let mut mult = 1.0;
    if blacks != 0.0 {
        let limit = 0.05;
        if safe_luma < limit {
            let x = safe_luma / limit;
            let mask = (1.0 - x) * (1.0 - x);
            let factor = 2.0_f32.powf(blacks * 0.75).min(3.9);
            mult *= mix(1.0, factor, mask);
        }
    }
    if shadows != 0.0 {
        let limit = 0.1;
        if safe_luma < limit {
            let x = safe_luma / limit;
            let mask = (1.0 - x) * (1.0 - x);
            let factor = 2.0_f32.powf(shadows * 1.5).min(3.9);
            mult *= mix(1.0, factor, mask);
        }
    }
    mult
}

// Ported from RapidRAW's tonal adjustment shader, with the preview blur supplied
// by a small CPU neighborhood average for the fallback path.
fn apply_tonal_adjustments(
    mut color: [f32; 3],
    blurred: [f32; 3],
    contrast: f32,
    shadows: f32,
    whites: f32,
    blacks: f32,
) -> [f32; 3] {
    let mut blurred_linear = blurred;
    if whites != 0.0 {
        let white_level = 1.0 - whites * 0.25;
        let w_mult = 1.0 / white_level.max(0.01);
        for c in &mut color {
            *c *= w_mult;
        }
        for c in &mut blurred_linear {
            *c *= w_mult;
        }
    }

    let pixel_luma = luma(color.map(|v| v.max(0.0)));
    let blurred_luma = luma(blurred_linear.map(|v| v.max(0.0)));
    let edge_diff = pixel_luma.max(0.0001).sqrt() - blurred_luma.max(0.0001).sqrt();
    let halo_protection = smoothstep(0.05, 0.25, edge_diff.abs());

    if shadows != 0.0 || blacks != 0.0 {
        let spatial_mult = shadow_mult(blurred_luma, shadows, blacks);
        let pixel_mult = shadow_mult(pixel_luma, shadows, blacks);
        let final_mult = mix(spatial_mult, pixel_mult, halo_protection);
        for c in &mut color {
            *c *= final_mult;
        }
    }

    if contrast != 0.0 {
        let strength = 2.0_f32.powf(contrast * 1.25);
        let gamma = 2.2;
        for c in &mut color {
            let safe = c.max(0.0);
            let perceptual = safe.powf(1.0 / gamma).clamp(0.0, 1.0);
            let curved = if perceptual < 0.5 {
                0.5 * (2.0 * perceptual).powf(strength)
            } else {
                1.0 - 0.5 * (2.0 * (1.0 - perceptual)).powf(strength)
            };
            let adjusted = curved.powf(gamma);
            let t = smoothstep(1.0, 1.01, safe);
            *c = mix(adjusted, *c, t);
        }
    }

    color
}

fn apply_highlights_adjustment(color: [f32; 3], highlights: f32) -> [f32; 3] {
    if highlights == 0.0 {
        return color;
    }
    let pixel_luma = luma(color.map(|v| v.max(0.0)));
    let highlight_mask = smoothstep(0.3, 0.95, (pixel_luma.max(0.0001) * 1.5).tanh());
    if highlight_mask < 0.001 {
        return color;
    }

    let new_color = if highlights < 0.0 {
        let new_luma = if pixel_luma <= 1.0 {
            pixel_luma.powf(1.0 - highlights * 1.75)
        } else {
            let excess = pixel_luma - 1.0;
            1.0 + excess / (1.0 + excess * -highlights * 6.0)
        };
        let scaled = color.map(|v| v * (new_luma / pixel_luma.max(0.0001)));
        let desat = smoothstep(1.0, 10.0, pixel_luma);
        mix3(scaled, [new_luma; 3], desat)
    } else {
        let factor = 2.0_f32.powf(highlights * 1.75);
        [color[0] * factor, color[1] * factor, color[2] * factor]
    };

    mix3(color, new_color, highlight_mask)
}

fn apply_white_balance(color: [f32; 3], temperature: f32, tint: f32) -> [f32; 3] {
    [
        color[0] * (1.0 + temperature * 0.2) * (1.0 + tint * 0.25),
        color[1] * (1.0 + temperature * 0.05) * (1.0 - tint * 0.25),
        color[2] * (1.0 - temperature * 0.2) * (1.0 + tint * 0.25),
    ]
}

fn apply_creative_color(color: [f32; 3], saturation: f32, vibrance: f32) -> [f32; 3] {
    let mut processed = color;
    let lum = luma(processed);

    if saturation != 0.0 {
        processed = [
            mix(lum, processed[0], 1.0 + saturation),
            mix(lum, processed[1], 1.0 + saturation),
            mix(lum, processed[2], 1.0 + saturation),
        ];
    }
    if vibrance == 0.0 {
        return processed;
    }

    let c_max = processed[0].max(processed[1]).max(processed[2]);
    let c_min = processed[0].min(processed[1]).min(processed[2]);
    let delta = c_max - c_min;
    if delta < 0.02 {
        return processed;
    }

    let current_sat = delta / c_max.max(0.001);
    let amount = if vibrance > 0.0 {
        let sat_mask = 1.0 - smoothstep(0.4, 0.9, current_sat);
        vibrance * sat_mask * 3.0
    } else {
        let desat_mask = 1.0 - smoothstep(0.2, 0.8, current_sat);
        vibrance * desat_mask
    };

    [
        mix(lum, processed[0], 1.0 + amount),
        mix(lum, processed[1], 1.0 + amount),
        mix(lum, processed[2], 1.0 + amount),
    ]
}

fn apply_clarity(color: [f32; 3], clarity: f32) -> [f32; 3] {
    if clarity == 0.0 {
        return color;
    }
    let lum = luma(color);
    let factor = 1.0 + clarity * 0.18;
    [
        lum + (color[0] - lum) * factor,
        lum + (color[1] - lum) * factor,
        lum + (color[2] - lum) * factor,
    ]
}

fn apply_sharpness(color: [f32; 3], blurred: [f32; 3], sharpness: f32) -> [f32; 3] {
    if sharpness <= 0.0 {
        return color;
    }
    let amount = sharpness * 0.65;
    [
        (color[0] + (color[0] - blurred[0]) * amount).max(0.0),
        (color[1] + (color[1] - blurred[1]) * amount).max(0.0),
        (color[2] + (color[2] - blurred[2]) * amount).max(0.0),
    ]
}

fn apply_noise_reduction(color: [f32; 3], blurred: [f32; 3], amount: f32) -> [f32; 3] {
    let amount = amount.clamp(0.0, 1.0);
    if amount <= 0.0 {
        return color;
    }
    mix3(color, blurred, amount * 0.75)
}
