use image::{
    DynamicImage, ExtendedColorType, GenericImageView, ImageBuffer, ImageEncoder, Rgba,
    codecs::png::{CompressionType, FilterType, PngEncoder},
};
use std::io::Cursor;
use wasm_bindgen::prelude::*;

const FROSTED_BG_MAX_SIDE: u32 = 2048;

fn as_js_error(context: &str, err: impl std::fmt::Display) -> JsValue {
    JsValue::from_str(&format!("[{context}] {err}"))
}

fn load_image(image_data: &[u8]) -> Result<DynamicImage, JsValue> {
    let image = image::load_from_memory(image_data)
        .map_err(|e| as_js_error("decode", format!("Failed to load image: {e}")))?;
    Ok(image)
}

fn encode_image(
    image: &ImageBuffer<Rgba<u8>, Vec<u8>>,
    _quality_hint: u8,
) -> Result<Vec<u8>, JsValue> {
    let mut cursor = Cursor::new(Vec::new());
    let (width, height) = image.dimensions();

    // Always emit PNG to preserve alpha channel and avoid output-format ambiguity.
    // Fast compression makes border generation noticeably quicker for large images.
    let encoder =
        PngEncoder::new_with_quality(&mut cursor, CompressionType::Fast, FilterType::NoFilter);
    encoder
        .write_image(image, width, height, ExtendedColorType::Rgba8)
        .map_err(|e| as_js_error("encode", format!("Failed to encode PNG: {e}")))?;

    Ok(cursor.into_inner())
}

fn process_image<F>(image_data: &[u8], jpeg_quality: u8, processor: F) -> Result<Vec<u8>, JsValue>
where
    F: FnOnce(DynamicImage) -> Result<ImageBuffer<Rgba<u8>, Vec<u8>>, JsValue>,
{
    let image = load_image(image_data)?;
    let processed = processor(image)?;
    encode_image(&processed, jpeg_quality)
}

fn apply_rounded_corners(buffer: &mut ImageBuffer<Rgba<u8>, Vec<u8>>, radius: u32) {
    if radius == 0 {
        return;
    }

    let (width, height) = buffer.dimensions();
    if width == 0 || height == 0 {
        return;
    }

    let clamped_radius = radius.min(width / 2).min(height / 2);
    if clamped_radius == 0 {
        return;
    }

    let radius_i64 = i64::from(clamped_radius);
    let radius_sq = radius_i64 * radius_i64;
    let transparent = Rgba([0, 0, 0, 0]);

    for y in 0..height {
        for x in 0..width {
            let dx = if x < clamped_radius {
                i64::from(clamped_radius - x)
            } else if x >= width - clamped_radius {
                i64::from(x - (width - clamped_radius))
            } else {
                0
            };

            let dy = if y < clamped_radius {
                i64::from(clamped_radius - y)
            } else if y >= height - clamped_radius {
                i64::from(y - (height - clamped_radius))
            } else {
                0
            };

            if dx > 0 && dy > 0 && (dx * dx + dy * dy > radius_sq) {
                buffer.put_pixel(x, y, transparent);
            }
        }
    }
}

fn compute_downscaled_dims(width: u32, height: u32, max_side: u32) -> (u32, u32, f32) {
    let current_max = width.max(height);
    if current_max <= max_side || current_max == 0 {
        return (width.max(1), height.max(1), 1.0);
    }

    let scale = max_side as f32 / current_max as f32;
    let scaled_w = ((width as f32) * scale).round().max(1.0) as u32;
    let scaled_h = ((height as f32) * scale).round().max(1.0) as u32;
    (scaled_w, scaled_h, scale)
}

/// Add a solid-color border around the input image.
#[wasm_bindgen]
pub fn add_colored_border(
    image_data: &[u8],
    border_width: u32,
    r: u8,
    g: u8,
    b: u8,
    jpeg_quality: u8,
) -> Result<Vec<u8>, JsValue> {
    process_image(image_data, jpeg_quality, |image| {
        let (width, height) = image.dimensions();
        let expanded = border_width.saturating_mul(2);
        let new_width = width.saturating_add(expanded);
        let new_height = height.saturating_add(expanded);
        let border_color = Rgba([r, g, b, 255]);

        let mut bordered = ImageBuffer::from_pixel(new_width, new_height, border_color);
        image::imageops::overlay(
            &mut bordered,
            &image,
            i64::from(border_width),
            i64::from(border_width),
        );
        Ok(bordered)
    })
}

/// Add a vignette effect on the whole image.
#[wasm_bindgen]
pub fn add_vignette_border(
    image_data: &[u8],
    strength: f32,
    jpeg_quality: u8,
) -> Result<Vec<u8>, JsValue> {
    process_image(image_data, jpeg_quality, |image| {
        let (width, height) = image.dimensions();
        let center_x = width as f32 * 0.5;
        let center_y = height as f32 * 0.5;
        let max_dist_sq = center_x * center_x + center_y * center_y;
        let strength = strength.clamp(0.0, 1.0);

        let mut buffer = image.to_rgba8();
        for (x, y, pixel) in buffer.enumerate_pixels_mut() {
            let dx = x as f32 - center_x;
            let dy = y as f32 - center_y;
            let distance_factor = (dx * dx + dy * dy) / max_dist_sq;
            let factor = (1.0 - distance_factor * strength).clamp(0.0, 1.0);

            pixel[0] = (pixel[0] as f32 * factor) as u8;
            pixel[1] = (pixel[1] as f32 * factor) as u8;
            pixel[2] = (pixel[2] as f32 * factor) as u8;
        }

        Ok(buffer)
    })
}

/// Create a frosted-style border:
/// blur + darken background, round corners, then overlay scaled foreground.
#[wasm_bindgen]
pub fn create_frosted_border(
    image_data: &[u8],
    blur_sigma: f32,
    brightness_adjustment: i32,
    corner_radius: u32,
    jpeg_quality: u8,
) -> Result<Vec<u8>, JsValue> {
    process_image(image_data, jpeg_quality, |image| {
        let (width, height) = image.dimensions();
        let (bg_w, bg_h, downscale_ratio) =
            compute_downscaled_dims(width, height, FROSTED_BG_MAX_SIDE);

        let blur_sigma = blur_sigma.max(0.0);
        let mut background = if downscale_ratio < 1.0 {
            let scaled =
                image::imageops::resize(&image, bg_w, bg_h, image::imageops::FilterType::Triangle);
            let scaled_sigma = (blur_sigma * downscale_ratio).max(0.5);
            let blurred_scaled = if blur_sigma <= 0.01 {
                scaled
            } else {
                image::imageops::blur(&scaled, scaled_sigma)
            };
            image::imageops::resize(
                &blurred_scaled,
                width,
                height,
                image::imageops::FilterType::Triangle,
            )
        } else if blur_sigma <= 0.01 {
            image.to_rgba8()
        } else {
            image::imageops::blur(&image, blur_sigma)
        };

        background = image::imageops::brighten(&background, brightness_adjustment);
        apply_rounded_corners(&mut background, corner_radius);

        let (orig_width, orig_height) = image.dimensions();

        let fg_width = ((orig_width as f32 * 0.75).max(1.0)) as u32;
        let fg_height = ((orig_height as f32 * 0.75).max(1.0)) as u32;

        // Triangle is noticeably faster than Lanczos3 in WASM while keeping acceptable quality.
        let foreground = image::imageops::resize(
            &image,
            fg_width,
            fg_height,
            image::imageops::FilterType::Triangle,
        );

        let offset_x = i64::from((width.saturating_sub(fg_width)) / 2);
        let offset_y = i64::from((height.saturating_sub(fg_height)) / 2);
        image::imageops::overlay(&mut background, &foreground, offset_x, offset_y);

        Ok(background)
    })
}
