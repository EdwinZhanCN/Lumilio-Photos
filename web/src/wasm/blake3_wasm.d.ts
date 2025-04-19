/* tslint:disable */
/* eslint-disable */
/**
 * Generates a BLAKE3 hash from any media asset buffer
 *
 * @param buffer - The raw bytes of the media file
 * @returns A hex-encoded BLAKE3 hash string
 */
export function hash_asset(buffer: Uint8Array): HashResult;
/**
 * Checks if two assets have the same hash
 *
 * @param buffer1 - The raw bytes of the first asset
 * @param buffer2 - The raw bytes of the second asset
 * @returns true if the hashes match, false otherwise
 */
export function compare_assets(buffer1: Uint8Array, buffer2: Uint8Array): boolean;
/**
 * Creates a HashResult from an existing hash string
 *
 * @param hash_string - A hex-encoded BLAKE3 hash string
 * @returns A HashResult object
 */
export function from_hash_string(hash_string: string): HashResult;
/**
 * Compares a buffer's hash with an existing hash string
 *
 * @param buffer - The raw bytes of the asset
 * @param hash_string - A hex-encoded BLAKE3 hash string to compare against
 * @returns true if the hashes match, false otherwise
 */
export function verify_asset_hash(buffer: Uint8Array, hash_string: string): boolean;
export class HashResult {
  free(): void;
  constructor(hash_string: string);
  readonly hash: string;
}

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
  readonly memory: WebAssembly.Memory;
  readonly __wbg_hashresult_free: (a: number, b: number) => void;
  readonly hashresult_hash: (a: number) => [number, number];
  readonly hashresult_new: (a: number, b: number) => number;
  readonly hash_asset: (a: number, b: number) => [number, number, number];
  readonly compare_assets: (a: number, b: number, c: number, d: number) => number;
  readonly from_hash_string: (a: number, b: number) => [number, number, number];
  readonly verify_asset_hash: (a: number, b: number, c: number, d: number) => [number, number, number];
  readonly __wbindgen_export_0: WebAssembly.Table;
  readonly __wbindgen_free: (a: number, b: number, c: number) => void;
  readonly __wbindgen_malloc: (a: number, b: number) => number;
  readonly __wbindgen_realloc: (a: number, b: number, c: number, d: number) => number;
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
