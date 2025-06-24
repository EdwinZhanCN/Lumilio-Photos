/* tslint:disable */
/* eslint-disable */
/**
 * 为图片添加纯色边框（重构版）
 */
export function add_colored_border(image_data: Uint8Array, border_width: number, r: number, g: number, b: number, jpeg_quality: number): Uint8Array;
/**
 * **新功能示例**: 为图片添加一个简单的“晕影”效果（暗角）作为边框
 */
export function add_vignette_border(image_data: Uint8Array, strength: number, jpeg_quality: number): Uint8Array;
/**
 * 创建一个“毛玻璃”效果的边框。
 *
 * # Arguments
 * * `image_data` - 原始图片数据。
 * * `blur_sigma` - 背景高斯模糊的强度，值越大越模糊 (例如: 15.0)。
 * * `brightness_adjustment` - 背景亮度调整，负数表示变暗 (例如: -40)。
 * * `corner_radius` - 背景的圆角半径 (例如: 30)。
 * * `jpeg_quality` - JPEG 输出质量。
 */
export function create_frosted_border(image_data: Uint8Array, blur_sigma: number, brightness_adjustment: number, corner_radius: number, jpeg_quality: number): Uint8Array;

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
  readonly memory: WebAssembly.Memory;
  readonly add_colored_border: (a: number, b: number, c: number, d: number, e: number, f: number, g: number) => [number, number, number, number];
  readonly add_vignette_border: (a: number, b: number, c: number, d: number) => [number, number, number, number];
  readonly create_frosted_border: (a: number, b: number, c: number, d: number, e: number, f: number) => [number, number, number, number];
  readonly __wbindgen_export_0: WebAssembly.Table;
  readonly __wbindgen_malloc: (a: number, b: number) => number;
  readonly __externref_table_dealloc: (a: number) => void;
  readonly __wbindgen_free: (a: number, b: number, c: number) => void;
  readonly __wbindgen_start: () => void;
}

export type SyncInitInput = BufferSource | WebAssembly.Module;
/**
* Instantiates the given `module`, which can either be bytes or
* a precompiled `WebAssembly.Module`.
*
* @param {{ module: SyncInitInput }} module - Passing `SyncInitInput` directly is deprecated.
*
* @returns {InitOutput}
*/
export function initSync(module: { module: SyncInitInput } | SyncInitInput): InitOutput;

/**
* If `module_or_path` is {RequestInfo} or {URL}, makes a request and
* for everything else, calls `WebAssembly.instantiate` directly.
*
* @param {{ module_or_path: InitInput | Promise<InitInput> }} module_or_path - Passing `InitInput` directly is deprecated.
*
* @returns {Promise<InitOutput>}
*/
export default function __wbg_init (module_or_path?: { module_or_path: InitInput | Promise<InitInput> } | InitInput | Promise<InitInput>): Promise<InitOutput>;
