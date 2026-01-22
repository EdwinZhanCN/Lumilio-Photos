use wasm_bindgen::prelude::*;
pub use wasm_bindgen_rayon::init_thread_pool;
use blake3::Hasher;

/// Fast single-pass hashing for small buffers.
#[wasm_bindgen]
pub fn hash_asset(buffer: &[u8]) -> String {
    blake3::hash(buffer).to_hex().to_string()
}

/// Streaming hasher for large files to maintain low memory usage.
#[wasm_bindgen]
pub struct StreamingHasher {
    inner: Hasher,
}

#[wasm_bindgen]
impl StreamingHasher {
    #[wasm_bindgen(constructor)]
    pub fn new() -> StreamingHasher {
        StreamingHasher {
            inner: Hasher::new(),
        }
    }

    /// Update the hasher with a chunk of data.
    pub fn update(&mut self, chunk: &[u8]) {
        self.inner.update(chunk);
    }

    /// Finalize the hash and return as a hex string.
    pub fn finalize(self) -> String {
        self.inner.finalize().to_hex().to_string()
    }

    /// Finalize the hash and return as raw bytes (32 bytes).
    #[wasm_bindgen(js_name = finalizeRaw)]
    pub fn finalize_raw(self) -> Vec<u8> {
        self.inner.finalize().as_bytes().to_vec()
    }
}

/// Verify if a buffer's hash matches the expected hex string.
#[wasm_bindgen]
pub fn verify_asset_hash(buffer: &[u8], expected_hex: &str) -> bool {
    let hash_bytes = blake3::hash(buffer);
    hash_bytes.to_hex().as_str() == expected_hex
}