# Source Vendor Artifacts

This folder stores the wasm-bindgen outputs consumed by `src/runner.ts`:

- `border_wasm.js`
- `border_wasm_bg.wasm`

Refresh from the Rust crate when `wasm/border-wasm/src/lib.rs` changes:

```bash
cd /Users/zhanzihao/Lumilio-Photos/wasm/border-wasm
wasm-pack build --target web --release --out-dir pkg --mode no-install --no-opt
cp pkg/border_wasm.js /Users/zhanzihao/Lumilio-Photos/plugins/lumilio-border-plugin/src/vendor/border_wasm.js
cp pkg/border_wasm_bg.wasm /Users/zhanzihao/Lumilio-Photos/plugins/lumilio-border-plugin/src/vendor/border_wasm_bg.wasm
```
