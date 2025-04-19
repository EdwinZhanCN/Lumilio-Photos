import * as wasm from "./thumbnail_wasm_bg.wasm";
export * from "./thumbnail_wasm_bg.js";
import { __wbg_set_wasm } from "./thumbnail_wasm_bg.js";
__wbg_set_wasm(wasm);
wasm.__wbindgen_start();
