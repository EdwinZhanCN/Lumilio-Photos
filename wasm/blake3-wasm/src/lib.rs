use wasm_bindgen::prelude::*;
use blake3::Hasher;

#[wasm_bindgen]
pub struct HashResult {
    hash: String,
}

#[wasm_bindgen]
impl HashResult {
    #[wasm_bindgen(getter)]
    pub fn hash(&self) -> String {
        self.hash.clone()
    }

    #[wasm_bindgen(constructor)]
    pub fn new(hash_string: String) -> HashResult {
        HashResult { hash: hash_string }
    }
}

/// Generates a BLAKE3 hash from any media asset buffer
///
/// @param buffer - The raw bytes of the media file
/// @returns A hex-encoded BLAKE3 hash string
#[wasm_bindgen]
pub fn hash_asset(buffer: &[u8]) -> Result<HashResult, JsError> {
    // Create a new hasher
    let mut hasher = Hasher::new();

    // Update with file contents
    hasher.update(buffer);

    // Finalize and get the hash
    let hash = hasher.finalize();

    // Convert to hex string
    let hash_hex = hash.to_hex().to_string();

    Ok(HashResult {
        hash: hash_hex,
    })
}

/// Checks if two assets have the same hash
///
/// @param buffer1 - The raw bytes of the first asset
/// @param buffer2 - The raw bytes of the second asset
/// @returns true if the hashes match, false otherwise
#[wasm_bindgen]
pub fn compare_assets(buffer1: &[u8], buffer2: &[u8]) -> bool {
    let mut hasher1 = Hasher::new();
    hasher1.update(buffer1);
    let hash1 = hasher1.finalize();

    let mut hasher2 = Hasher::new();
    hasher2.update(buffer2);
    let hash2 = hasher2.finalize();

    hash1 == hash2
}

/// Creates a HashResult from an existing hash string
///
/// @param hash_string - A hex-encoded BLAKE3 hash string
/// @returns A HashResult object
#[wasm_bindgen]
pub fn from_hash_string(hash_string: String) -> Result<HashResult, JsError> {
    // Validate that the input is a valid hex string of correct length
    if hash_string.len() != 64 || !hash_string.chars().all(|c| c.is_digit(16)) {
        return Err(JsError::new("Invalid hash string format"));
    }

    Ok(HashResult {
        hash: hash_string,
    })
}

/// Compares a buffer's hash with an existing hash string
///
/// @param buffer - The raw bytes of the asset
/// @param hash_string - A hex-encoded BLAKE3 hash string to compare against
/// @returns true if the hashes match, false otherwise
#[wasm_bindgen]
pub fn verify_asset_hash(buffer: &[u8], hash_string: &str) -> Result<bool, JsError> {
    if hash_string.len() != 64 || !hash_string.chars().all(|c| c.is_digit(16)) {
        return Err(JsError::new("Invalid hash string format"));
    }

    let result = hash_asset(buffer)?;
    Ok(result.hash() == hash_string)
}