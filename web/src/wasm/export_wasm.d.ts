/* tslint:disable */
 
export function get_supported_formats(): Array<any>;
export function validate_export_options(options_js: any): boolean;
export function greet(name: string): string;
export function create_blob(data: Uint8Array, mime_type: string): Blob;
export function get_memory_usage(): number;
export class ImageProcessor {
  free(): void;
  constructor();
  /**
   * Load image from byte array
   */
  load_from_bytes(bytes: Uint8Array): boolean;
  /**
   * Get image dimensions
   */
  get_dimensions(): Array<any> | undefined;
  /**
   * Process and export image with given options
   */
  export_image(options_js: any): any;
}

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
  readonly memory: WebAssembly.Memory;
  readonly __wbg_imageprocessor_free: (a: number, b: number) => void;
  readonly imageprocessor_new: () => number;
  readonly imageprocessor_load_from_bytes: (a: number, b: number, c: number) => number;
  readonly imageprocessor_get_dimensions: (a: number) => any;
  readonly imageprocessor_export_image: (a: number, b: any) => any;
  readonly get_supported_formats: () => any;
  readonly validate_export_options: (a: any) => number;
  readonly greet: (a: number, b: number) => [number, number];
  readonly create_blob: (a: number, b: number, c: number, d: number) => [number, number, number];
  readonly get_memory_usage: () => number;
  readonly __wbindgen_malloc: (a: number, b: number) => number;
  readonly __wbindgen_realloc: (a: number, b: number, c: number, d: number) => number;
  readonly __wbindgen_free: (a: number, b: number, c: number) => void;
  readonly __wbindgen_exn_store: (a: number) => void;
  readonly __externref_table_alloc: () => number;
  readonly __wbindgen_export_5: WebAssembly.Table;
  readonly __externref_table_dealloc: (a: number) => void;
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
