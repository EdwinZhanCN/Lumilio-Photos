use image::{
    DynamicImage,
    ExtendedColorType,
    GenericImageView,
    ImageBuffer,
    ImageEncoder,
    ImageFormat,
    Rgba,
    codecs::{jpeg::JpegEncoder, png::PngEncoder}, // <-- 新增：导入编码器
};
use std::io::Cursor;
use wasm_bindgen::prelude::*;

// (可以复用之前定义的 log 函数)
#[wasm_bindgen]
extern "C" {
    #[wasm_bindgen(js_namespace = console)]
    fn log(s: &str);
}

// ===================================================================================
// 1. 私有的、通用的图片处理“引擎”
//    这个函数处理所有重复的逻辑：加载、编码、错误处理。
//    它通过泛型 <F> 接受一个闭包 `processor`，这个闭包是真正的图片处理逻辑。
// ===================================================================================
// ===================================================================================
// 1. 私有的、通用的图片处理“引擎” (已最终修正)
// ===================================================================================
fn process_image<F>(image_data: &[u8], jpeg_quality: u8, processor: F) -> Result<Vec<u8>, JsValue>
where
    // **核心修正**: 在下面的 ImageBuffer 中，显式提供第二个泛型参数 Vec<u8>
    F: FnOnce(DynamicImage) -> Result<ImageBuffer<Rgba<u8>, Vec<u8>>, JsValue>,
{
    // --- 前置和核心处理逻辑 (这部分无变化) ---
    log("Engine: Guessing image format...");
    let input_format = image::guess_format(image_data)
        .map_err(|e| JsValue::from_str(&format!("[Engine] Could not guess format: {}", e)))?;

    log("Engine: Loading image...");
    let img = image::load_from_memory_with_format(image_data, input_format)
        .map_err(|e| JsValue::from_str(&format!("[Engine] Failed to load image: {}", e)))?;

    log("Engine: Handing over to a specific processor...");
    let processed_buffer = processor(img)?;

    // --- 后置逻辑 (编码部分已修正) ---
    log("Engine: Encoding final image using specific encoder...");
    let mut buf = Cursor::new(Vec::new());
    let (width, height) = processed_buffer.dimensions();

    // 根据输入格式选择对应的编码器
    match input_format {
        ImageFormat::Jpeg => {
            // **修正**: 声明为 mut，并调用 .write_image()
            let encoder = JpegEncoder::new_with_quality(&mut buf, jpeg_quality.clamp(1, 100));
            encoder
                .write_image(&processed_buffer, width, height, ExtendedColorType::Rgba8)
                .map_err(|e| {
                    JsValue::from_str(&format!("[Engine] Failed to encode JPEG: {}", e))
                })?;
        }
        ImageFormat::Png => {
            // **修正**: 声明为 mut，并调用 .write_image()
            let encoder = PngEncoder::new(&mut buf);
            encoder
                .write_image(&processed_buffer, width, height, ExtendedColorType::Rgba8)
                .map_err(|e| JsValue::from_str(&format!("[Engine] Failed to encode PNG: {}", e)))?;
        }
        _ => {
            // 后备方案
            log(&format!(
                "[Engine] Fallback to PNG for unsupported format {:?}",
                input_format
            ));
            // **修正**: 声明为 mut，并调用 .write_image()
            let encoder = PngEncoder::new(&mut buf);
            encoder
                .write_image(&processed_buffer, width, height, ExtendedColorType::Rgba8)
                .map_err(|e| {
                    JsValue::from_str(&format!("[Engine] Failed to encode PNG fallback: {}", e))
                })?;
        }
    };

    log("Engine: Processing complete.");
    Ok(buf.into_inner())
}

// ===================================================================================
// 2. 公开暴露给 WebAssembly 的函数
//    这些函数现在变得非常简洁。它们只定义自己的核心逻辑，然后调用通用引擎。
// ===================================================================================

/// 为图片添加纯色边框（重构版）
#[wasm_bindgen]
pub fn add_colored_border(
    image_data: &[u8],
    border_width: u32,
    r: u8,
    g: u8,
    b: u8,
    jpeg_quality: u8,
) -> Result<Vec<u8>, JsValue> {
    // 调用通用处理引擎，并传入一个定义了“如何添加纯色边框”的闭包。
    process_image(image_data, jpeg_quality, |img| {
        log("Processor: add_colored_border logic running...");
        let (width, height) = img.dimensions();
        let new_width = width + 2 * border_width;
        let new_height = height + 2 * border_width;
        let border_color = Rgba([r, g, b, 255u8]);

        let mut bordered_img_buffer = ImageBuffer::from_pixel(new_width, new_height, border_color);
        image::imageops::overlay(
            &mut bordered_img_buffer,
            &img,
            border_width as i64,
            border_width as i64,
        );

        // 闭包需要返回一个 Result
        Ok(bordered_img_buffer)
    })
}

/// **新功能示例**: 为图片添加一个简单的“晕影”效果（暗角）作为边框
#[wasm_bindgen]
pub fn add_vignette_border(
    image_data: &[u8],
    strength: f32, // 晕影强度 (0.0 to 1.0)
    jpeg_quality: u8,
) -> Result<Vec<u8>, JsValue> {
    process_image(image_data, jpeg_quality, |img| {
        log("Processor: add_vignette_border logic running...");
        let (width, height) = img.dimensions();
        let center_x = width as f32 / 2.0;
        let center_y = height as f32 / 2.0;
        let max_dist = (center_x.powi(2) + center_y.powi(2)).sqrt();
        let strength = strength.clamp(0.0, 1.0);

        // 直接在原图上修改像素
        let buffer = img.to_rgba8();
        let mut new_buffer = buffer.clone();

        for (x, y, pixel) in new_buffer.enumerate_pixels_mut() {
            let dx = x as f32 - center_x;
            let dy = y as f32 - center_y;
            let dist = (dx.powi(2) + dy.powi(2)).sqrt();
            let factor = 1.0 - (dist / max_dist).powf(2.0) * strength;

            pixel[0] = (pixel[0] as f32 * factor) as u8; // R
            pixel[1] = (pixel[1] as f32 * factor) as u8; // G
            pixel[2] = (pixel[2] as f32 * factor) as u8; // B
        }

        Ok(new_buffer)
    })
}

// ===================================================================================
// 新功能: 创建一个高斯模糊、变暗、带圆角的背景，并将原图缩小后置于其上
// ===================================================================================
/// 创建一个“毛玻璃”效果的边框。
///
/// # Arguments
/// * `image_data` - 原始图片数据。
/// * `blur_sigma` - 背景高斯模糊的强度，值越大越模糊 (例如: 15.0)。
/// * `brightness_adjustment` - 背景亮度调整，负数表示变暗 (例如: -40)。
/// * `corner_radius` - 背景的圆角半径 (例如: 30)。
/// * `jpeg_quality` - JPEG 输出质量。
#[wasm_bindgen]
pub fn create_frosted_border(
    image_data: &[u8],
    blur_sigma: f32,
    brightness_adjustment: i32,
    corner_radius: u32,
    jpeg_quality: u8,
) -> Result<Vec<u8>, JsValue> {
    process_image(image_data, jpeg_quality, |img| {
        log("Processor: create_frosted_border logic running...");

        // --- 步骤 1: 创建背景图 ---
        log("Step 1: Creating background (blur + darken)...");

        // 应用高斯模糊，注意 blur 返回的是 ImageBuffer
        let background_blurred = image::imageops::blur(&img, blur_sigma);

        // 降低亮度，注意 brighten 返回的也是新的 ImageBuffer
        // 我们将其声明为 mut，因为接下来要修改它（添加圆角）
        let mut background = image::imageops::brighten(&background_blurred, brightness_adjustment);

        let (width, height) = background.dimensions();

        // --- 步骤 2: 为背景图添加圆角 ---
        log("Step 2: Applying rounded corners to background...");
        let transparent = Rgba([0u8, 0u8, 0u8, 0u8]);
        let radius = corner_radius;
        let radius_sq = (radius as f32).powi(2);

        for y in 0..height {
            for x in 0..width {
                // 判断像素是否在四个角落的矩形区域内
                let is_in_top_left = x < radius && y < radius;
                let is_in_top_right = x >= width - radius && y < radius;
                let is_in_bottom_left = x < radius && y >= height - radius;
                let is_in_bottom_right = x >= width - radius && y >= height - radius;

                if is_in_top_left {
                    let dist_sq =
                        (radius as f32 - x as f32).powi(2) + (radius as f32 - y as f32).powi(2);
                    if dist_sq > radius_sq {
                        background.put_pixel(x, y, transparent);
                    }
                } else if is_in_top_right {
                    let dist_sq = (x as f32 - (width - radius) as f32).powi(2)
                        + (radius as f32 - y as f32).powi(2);
                    if dist_sq > radius_sq {
                        background.put_pixel(x, y, transparent);
                    }
                } else if is_in_bottom_left {
                    let dist_sq = (radius as f32 - x as f32).powi(2)
                        + (y as f32 - (height - radius) as f32).powi(2);
                    if dist_sq > radius_sq {
                        background.put_pixel(x, y, transparent);
                    }
                } else if is_in_bottom_right {
                    let dist_sq = (x as f32 - (width - radius) as f32).powi(2)
                        + (y as f32 - (height - radius) as f32).powi(2);
                    if dist_sq > radius_sq {
                        background.put_pixel(x, y, transparent);
                    }
                }
            }
        }

        // --- 步骤 3: 创建前景图 (缩小75%) ---
        log("Step 3: Creating foreground (resizing original)...");
        let (orig_width, orig_height) = img.dimensions();
        let fg_width = (orig_width as f32 * 0.75) as u32;
        let fg_height = (orig_height as f32 * 0.75) as u32;

        // 使用 Lanczos3 滤波器进行高质量的缩放
        let foreground = image::imageops::resize(
            &img,
            fg_width,
            fg_height,
            image::imageops::FilterType::Lanczos3,
        );

        // --- 步骤 4: 合成图像 ---
        log("Step 4: Overlaying foreground onto background...");
        let offset_x = ((width - fg_width) / 2) as i64;
        let offset_y = ((height - fg_height) / 2) as i64;

        image::imageops::overlay(&mut background, &foreground, offset_x, offset_y);

        Ok(background)
    })
}
