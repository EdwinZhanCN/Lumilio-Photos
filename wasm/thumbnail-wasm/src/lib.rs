use wasm_bindgen::prelude::*;
use image::{ImageFormat, ImageReader};
use std::io::Cursor;

#[wasm_bindgen]
pub struct ThumbnailResult {
    width: u32,
    height: u32,
    data: Vec<u8>,
}

#[wasm_bindgen]
impl ThumbnailResult {
    #[wasm_bindgen(getter)]
    pub fn width(&self) -> u32 { self.width }

    #[wasm_bindgen(getter)]
    pub fn height(&self) -> u32 { self.height }

    #[wasm_bindgen(getter)]
    pub fn data(&self) -> Vec<u8> { self.data.clone() }
}


#[wasm_bindgen]
pub fn generate_thumbnail(buffer: &[u8], max_size: u32) -> Result<Vec<u8>, JsError> {
    let img = match ImageReader::new(Cursor::new(buffer))
        .with_guessed_format()?
        .decode()
    {
        Ok(img) => img,
        Err(e) => return Err(JsError::new(&format!("Decode error: {}", e)))
    };

    let (width, height) = calculate_size(img.width(), img.height(), max_size);
    let thumbnail = img.thumbnail(width, height);

    // Convert to RGB8 which is supported by JPEG encoder
    let rgb_image = thumbnail.into_rgb8();

    let mut output = Cursor::new(Vec::new());
    rgb_image.write_to(&mut output, ImageFormat::Jpeg)
        .map_err(|e| JsError::new(&format!("Encode error: {}", e)))?;

    Ok(output.into_inner())
}

fn calculate_size(orig_w: u32, orig_h: u32, max_size: u32) -> (u32, u32) {
    let ratio = orig_w as f32 / orig_h as f32;
    if orig_w > orig_h {
        (max_size, (max_size as f32 / ratio) as u32)
    } else {
        ((max_size as f32 * ratio) as u32, max_size)
    }
}