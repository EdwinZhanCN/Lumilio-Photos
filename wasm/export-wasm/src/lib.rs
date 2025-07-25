mod utils;

use image::{
    codecs::jpeg::JpegEncoder, codecs::png::PngEncoder, codecs::webp::WebPEncoder,
    imageops::FilterType, DynamicImage, ExtendedColorType, ImageEncoder,
};
use js_sys::{Array, Uint8Array};
use serde::{Deserialize, Serialize};
use wasm_bindgen::prelude::*;
use web_sys::{Blob, BlobPropertyBag};

// When the `wee_alloc` feature is enabled, use `wee_alloc` as the global
// allocator.
#[cfg(feature = "wee_alloc")]
#[global_allocator]
static ALLOC: wee_alloc::WeeAlloc = wee_alloc::WeeAlloc::INIT;

// Import the `console.log` function from the browser environment
#[wasm_bindgen]
extern "C" {
    #[wasm_bindgen(js_namespace = console)]
    fn log(s: &str);

    #[wasm_bindgen(js_namespace = console)]
    fn error(s: &str);
}

// A macro to provide `println!(..)`-style syntax for `console.log` logging.
macro_rules! console_log {
    ( $( $t:tt )* ) => {
        web_sys::console::log_1(&format!( $( $t )* ).into())
    }
}

macro_rules! console_error {
    ( $( $t:tt )* ) => {
        web_sys::console::error_1(&format!( $( $t )* ).into())
    }
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ExportOptions {
    pub format: String, // "jpeg", "png", "webp", "original"
    pub quality: f32,   // 0.1 to 1.0 for lossy formats
    pub max_width: Option<u32>,
    pub max_height: Option<u32>,
    pub filename: Option<String>,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct ExportResult {
    pub success: bool,
    pub data: Option<Vec<u8>>,
    pub filename: Option<String>,
    pub error: Option<String>,
    pub width: u32,
    pub height: u32,
}

#[wasm_bindgen]
pub struct ImageProcessor {
    image: Option<DynamicImage>,
}

#[wasm_bindgen]
impl ImageProcessor {
    #[wasm_bindgen(constructor)]
    pub fn new() -> ImageProcessor {
        utils::set_panic_hook();
        console_log!("ImageProcessor initialized");

        ImageProcessor { image: None }
    }

    /// Load image from byte array
    #[wasm_bindgen]
    pub fn load_from_bytes(&mut self, bytes: &[u8]) -> bool {
        match image::load_from_memory(bytes) {
            Ok(img) => {
                console_log!(
                    "Image loaded successfully: {}x{}",
                    img.width(),
                    img.height()
                );
                self.image = Some(img);
                true
            }
            Err(e) => {
                console_error!("Failed to load image: {}", e);
                false
            }
        }
    }

    /// Get image dimensions
    #[wasm_bindgen]
    pub fn get_dimensions(&self) -> Option<Array> {
        if let Some(ref img) = self.image {
            let dimensions = Array::new();
            dimensions.set(0, JsValue::from(img.width()));
            dimensions.set(1, JsValue::from(img.height()));
            Some(dimensions)
        } else {
            None
        }
    }

    /// Process and export image with given options
    #[wasm_bindgen]
    pub fn export_image(&self, options_js: &JsValue) -> JsValue {
        let options: ExportOptions = match serde_wasm_bindgen::from_value(options_js.clone()) {
            Ok(opts) => opts,
            Err(e) => {
                console_error!("Failed to parse export options: {}", e);
                return serde_wasm_bindgen::to_value(&ExportResult {
                    success: false,
                    data: None,
                    filename: None,
                    error: Some(format!("Invalid options: {}", e)),
                    width: 0,
                    height: 0,
                })
                .unwrap();
            }
        };

        if let Some(ref img) = self.image {
            match self.process_image(img.clone(), &options) {
                Ok(result) => {
                    console_log!(
                        "Image export successful: {} bytes",
                        result.data.as_ref().map_or(0, |d| d.len())
                    );
                    serde_wasm_bindgen::to_value(&result).unwrap()
                }
                Err(e) => {
                    console_error!("Image export failed: {}", e);
                    serde_wasm_bindgen::to_value(&ExportResult {
                        success: false,
                        data: None,
                        filename: None,
                        error: Some(e),
                        width: 0,
                        height: 0,
                    })
                    .unwrap()
                }
            }
        } else {
            console_error!("No image loaded");
            serde_wasm_bindgen::to_value(&ExportResult {
                success: false,
                data: None,
                filename: None,
                error: Some("No image loaded".to_string()),
                width: 0,
                height: 0,
            })
            .unwrap()
        }
    }

    fn process_image(
        &self,
        mut img: DynamicImage,
        options: &ExportOptions,
    ) -> Result<ExportResult, String> {
        // Resize if needed
        if let (Some(max_width), Some(max_height)) = (options.max_width, options.max_height) {
            img = self.resize_image(img, max_width, max_height);
        } else if let Some(max_width) = options.max_width {
            let aspect_ratio = img.height() as f32 / img.width() as f32;
            let new_height = (max_width as f32 * aspect_ratio) as u32;
            img = img.resize(max_width, new_height, FilterType::Lanczos3);
        } else if let Some(max_height) = options.max_height {
            let aspect_ratio = img.width() as f32 / img.height() as f32;
            let new_width = (max_height as f32 * aspect_ratio) as u32;
            img = img.resize(new_width, max_height, FilterType::Lanczos3);
        }

        let (width, height) = (img.width(), img.height());

        // Convert to bytes based on format
        let data = match options.format.to_lowercase().as_str() {
            "jpeg" | "jpg" => self.encode_jpeg(&img, options.quality)?,
            "png" => self.encode_png(&img)?,
            "webp" => self.encode_webp(&img, options.quality)?,
            "original" => {
                // For original, we would need the original bytes
                // This is a simplified version that converts to PNG
                self.encode_png(&img)?
            }
            _ => return Err(format!("Unsupported format: {}", options.format)),
        };

        let filename = options.filename.clone().unwrap_or_else(|| {
            let extension = match options.format.to_lowercase().as_str() {
                "jpeg" | "jpg" => "jpg",
                "png" => "png",
                "webp" => "webp",
                _ => "jpg",
            };
            format!("lumilio-export.{}", extension)
        });

        Ok(ExportResult {
            success: true,
            data: Some(data),
            filename: Some(filename),
            error: None,
            width,
            height,
        })
    }

    fn resize_image(&self, img: DynamicImage, max_width: u32, max_height: u32) -> DynamicImage {
        let (width, height) = (img.width(), img.height());

        let width_ratio = max_width as f32 / width as f32;
        let height_ratio = max_height as f32 / height as f32;

        let ratio = width_ratio.min(height_ratio);

        if ratio < 1.0 {
            let new_width = (width as f32 * ratio) as u32;
            let new_height = (height as f32 * ratio) as u32;
            img.resize(new_width, new_height, FilterType::Lanczos3)
        } else {
            img
        }
    }

    fn encode_jpeg(&self, img: &DynamicImage, quality: f32) -> Result<Vec<u8>, String> {
        let mut buffer = Vec::new();
        let quality_u8 = (quality * 100.0).clamp(1.0, 100.0) as u8;

        let mut encoder = JpegEncoder::new_with_quality(&mut buffer, quality_u8);

        match img.color() {
            image::ColorType::Rgb8 => {
                encoder
                    .encode(
                        img.as_rgb8().unwrap().as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgb8,
                    )
                    .map_err(|e| format!("JPEG encoding error: {}", e))?;
            }
            _ => {
                let rgb_img = img.to_rgb8();
                encoder
                    .encode(
                        rgb_img.as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgb8,
                    )
                    .map_err(|e| format!("JPEG encoding error: {}", e))?;
            }
        }

        Ok(buffer)
    }

    fn encode_png(&self, img: &DynamicImage) -> Result<Vec<u8>, String> {
        let mut buffer = Vec::new();
        let encoder = PngEncoder::new(&mut buffer);

        match img.color() {
            image::ColorType::Rgba8 => {
                encoder
                    .write_image(
                        img.as_rgba8().unwrap().as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgba8,
                    )
                    .map_err(|e| format!("PNG encoding error: {}", e))?;
            }
            image::ColorType::Rgb8 => {
                encoder
                    .write_image(
                        img.as_rgb8().unwrap().as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgb8,
                    )
                    .map_err(|e| format!("PNG encoding error: {}", e))?;
            }
            _ => {
                let rgba_img = img.to_rgba8();
                encoder
                    .write_image(
                        rgba_img.as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgba8,
                    )
                    .map_err(|e| format!("PNG encoding error: {}", e))?;
            }
        }

        Ok(buffer)
    }

    fn encode_webp(&self, img: &DynamicImage, quality: f32) -> Result<Vec<u8>, String> {
        let mut buffer = Vec::new();
        let _quality_f32 = quality * 100.0;

        let encoder = WebPEncoder::new_lossless(&mut buffer);

        match img.color() {
            image::ColorType::Rgba8 => {
                encoder
                    .encode(
                        img.as_rgba8().unwrap().as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgba8,
                    )
                    .map_err(|e| format!("WebP encoding error: {}", e))?;
            }
            image::ColorType::Rgb8 => {
                encoder
                    .encode(
                        img.as_rgb8().unwrap().as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgb8,
                    )
                    .map_err(|e| format!("WebP encoding error: {}", e))?;
            }
            _ => {
                let rgba_img = img.to_rgba8();
                encoder
                    .encode(
                        rgba_img.as_raw(),
                        img.width(),
                        img.height(),
                        ExtendedColorType::Rgba8,
                    )
                    .map_err(|e| format!("WebP encoding error: {}", e))?;
            }
        }

        Ok(buffer)
    }
}

// Utility functions that can be called directly
#[wasm_bindgen]
pub fn get_supported_formats() -> Array {
    let formats = Array::new();
    formats.set(0, JsValue::from_str("jpeg"));
    formats.set(1, JsValue::from_str("png"));
    formats.set(2, JsValue::from_str("webp"));
    formats.set(3, JsValue::from_str("original"));
    formats
}

#[wasm_bindgen]
pub fn validate_export_options(options_js: &JsValue) -> bool {
    match serde_wasm_bindgen::from_value::<ExportOptions>(options_js.clone()) {
        Ok(options) => {
            // Validate format
            let valid_formats = ["jpeg", "jpg", "png", "webp", "original"];
            if !valid_formats.contains(&options.format.to_lowercase().as_str()) {
                return false;
            }

            // Validate quality
            if options.quality < 0.1 || options.quality > 1.0 {
                return false;
            }

            // Validate dimensions
            if let Some(width) = options.max_width {
                if width == 0 || width > 16384 {
                    return false;
                }
            }

            if let Some(height) = options.max_height {
                if height == 0 || height > 16384 {
                    return false;
                }
            }

            true
        }
        Err(_) => false,
    }
}

// Simple function to test WASM loading
#[wasm_bindgen]
pub fn greet(name: &str) -> String {
    format!("Hello, {}! Export WASM module is ready.", name)
}

// Function to create a Blob from bytes (helper for JavaScript)
#[wasm_bindgen]
pub fn create_blob(data: &[u8], mime_type: &str) -> Result<Blob, JsValue> {
    let uint8_array = Uint8Array::new_with_length(data.len() as u32);
    uint8_array.copy_from(data);

    let blob_parts = Array::new();
    blob_parts.set(0, uint8_array.into());

    let blob_property_bag = BlobPropertyBag::new();
    blob_property_bag.set_type(mime_type);

    Blob::new_with_u8_array_sequence_and_options(&blob_parts, &blob_property_bag)
}

// Memory management helper
#[wasm_bindgen]
pub fn get_memory_usage() -> u32 {
    // This is a simplified version - in practice you might want more detailed memory info
    std::mem::size_of::<ImageProcessor>() as u32
}
