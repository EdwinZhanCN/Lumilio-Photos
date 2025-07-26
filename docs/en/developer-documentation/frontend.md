# Frontend Tech Stack

## React.js
### Redux

## Vite
### Vitest

## TailwindCSS
### DaisyUI

## WebAssembly (WASM)
WebAssembly (abbreviated Wasm) is a binary instruction format for a stack-based virtual machine. Wasm is designed as a portable compilation target for programming languages, enabling deployment on the web for client and server applications.

For more information, see https://webassembly.org

This project uses `wasm-pack` to package the **Rust** project into `.wasm` and `.js` files for the frontend **WebWorker** to call.

### BLAKE3 Hash
- **Much faster** than MD5, SHA-1, SHA-2, SHA-3, and BLAKE2.
- **Secure**, unlike MD5 and SHA-1. And secure against length extension,
  unlike SHA-2.
- **Highly parallelizable** across any number of threads and SIMD lanes,
  because it's a Merkle tree on the inside.
- Capable of **verified streaming** and **incremental updates**, again
  because it's a Merkle tree.
- A **PRF**, **MAC**, **KDF**, and **XOF**, as well as a regular hash.
- **One algorithm with no variants**, which is fast on x86-64 and also
  on smaller architectures.

For more information, see https://github.com/BLAKE3-team/BLAKE3

This project uses WebAssembly to call BLAKE3 to hash media uploaded by users in the frontend (client browser).

### Performance

In our testing environment with mixed media file types (JPEG, RAW, and PXD - Pixelmator Pro files), the BLAKE3 WASM implementation achieved the following performance metrics:

- **Processing Speed**: Even in very conservative setting, it reaches **515 MB/s in 173 files (2.94 GB total)**
- **Memory Usage**: The memory usage is **~28.9MB** for the entire process.
- **Setting**:
```js [src/worker/hash.worker.js]
    const CONCURRENCY = assets[0]?.size > 100_000_000 ? 2 : 4; // 4 threds
    //...
```
- **Environment**: MacOS 15.4 M2 Pro (4E6P) Client Browser *(Chrome Version 135.0.7049.43 (Official Build) (arm64))*
- **Test Files**: Mix of different image formats
  - JPEG photographs **(5-15 MB each)**
  - RAW camera files **(20-50 MB each)**
  - PXD (Pixelmator Pro) files **(30-50 MB each)**
- **Resource**: [json trace file](/assets/ChromeVercelApr13.json) from Chrome DevTools

This performance demonstrates efficient client-side hashing capabilities, enabling quick duplicate detection and file verification without server-side processing.

**Important Functions**

`HashResult`

```js [src/wasm/blake3_wasm.js]
export class HashResult {
    // Encapsulates the hash result
    // The hash is a 256-bit BLAKE3 hash, represented as a hex string
    get hash() { /* Get the hash string */ }
    constructor(hash_string) { /* Create an instance from a string */ }
    free() { /* Free WASM memory */ }
}
```

`hash_asset`

```js [src/wasm/blake3_wasm.js]
export function hash_asset(buffer) {
    // ...
    // Generates a BLAKE3 hash from any media asset buffer
    // Return HashResult Object
}
```

`compare_assets`

```js [src/wasm/blake3_wasm.js]
export function compare_assets(buffer1, buffer2) {
    // ...
    // Directly compares the hashes of two binary buffers
    // Returns a boolean value
}
```

`verify_asset_hash`

```js [src/wasm/blake3_wasm.js]
export function verify_asset_hash(buffer, hash_string) {
    // ...
    // Verifies if the binary buffer matches an existing hash string
    // Returns a boolean value
}
```

