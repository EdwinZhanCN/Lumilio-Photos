## WASM

Build and synchronize the browser BLAKE3 package from the repository root:

```shell
task wasm:blake3
```

The browser hasher is intentionally single-threaded. Do not enable Rayon,
atomics, or shared memory: upload hashing must work on LAN HTTP origins where
`SharedArrayBuffer` is unavailable.
