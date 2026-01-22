/* tslint:disable */
/* eslint-disable */

export class StreamingHasher {
  free(): void;
  [Symbol.dispose](): void;
  /**
   * Finalize the hash and return as raw bytes (32 bytes).
   */
  finalizeRaw(): Uint8Array;
  constructor();
  /**
   * Update the hasher with a chunk of data.
   */
  update(chunk: Uint8Array): void;
  /**
   * Finalize the hash and return as a hex string.
   */
  finalize(): string;
}

/**
 * Fast single-pass hashing for small buffers.
 */
export function hash_asset(buffer: Uint8Array): string;

export function initThreadPool(num_threads: number): Promise<any>;

/**
 * Verify if a buffer's hash matches the expected hex string.
 */
export function verify_asset_hash(buffer: Uint8Array, expected_hex: string): boolean;

export class wbg_rayon_PoolBuilder {
  private constructor();
  free(): void;
  [Symbol.dispose](): void;
  numThreads(): number;
  build(): void;
  receiver(): number;
}

export function wbg_rayon_start_worker(receiver: number): void;

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
  readonly __wbg_streaminghasher_free: (a: number, b: number) => void;
  readonly hash_asset: (a: number, b: number) => [number, number];
  readonly streaminghasher_finalize: (a: number) => [number, number];
  readonly streaminghasher_finalizeRaw: (a: number) => [number, number];
  readonly streaminghasher_new: () => number;
  readonly streaminghasher_update: (a: number, b: number, c: number) => void;
  readonly verify_asset_hash: (a: number, b: number, c: number, d: number) => number;
  readonly __wbg_wbg_rayon_poolbuilder_free: (a: number, b: number) => void;
  readonly wbg_rayon_poolbuilder_build: (a: number) => void;
  readonly wbg_rayon_poolbuilder_numThreads: (a: number) => number;
  readonly wbg_rayon_poolbuilder_receiver: (a: number) => number;
  readonly wbg_rayon_start_worker: (a: number) => void;
  readonly initThreadPool: (a: number) => any;
  readonly memory: WebAssembly.Memory;
  readonly __wbindgen_exn_store: (a: number) => void;
  readonly __externref_table_alloc: () => number;
  readonly __wbindgen_externrefs: WebAssembly.Table;
  readonly __wbindgen_free: (a: number, b: number, c: number) => void;
  readonly __wbindgen_malloc: (a: number, b: number) => number;
  readonly __wbindgen_realloc: (a: number, b: number, c: number, d: number) => number;
  readonly __wbindgen_thread_destroy: (a?: number, b?: number, c?: number) => void;
  readonly __wbindgen_start: (a: number) => void;
}

export type SyncInitInput = BufferSource | WebAssembly.Module;

/**
* Instantiates the given `module`, which can either be bytes or
* a precompiled `WebAssembly.Module`.
*
* @param {{ module: SyncInitInput, memory?: WebAssembly.Memory, thread_stack_size?: number }} module - Passing `SyncInitInput` directly is deprecated.
* @param {WebAssembly.Memory} memory - Deprecated.
*
* @returns {InitOutput}
*/
export function initSync(module: { module: SyncInitInput, memory?: WebAssembly.Memory, thread_stack_size?: number } | SyncInitInput, memory?: WebAssembly.Memory): InitOutput;

/**
* If `module_or_path` is {RequestInfo} or {URL}, makes a request and
* for everything else, calls `WebAssembly.instantiate` directly.
*
* @param {{ module_or_path: InitInput | Promise<InitInput>, memory?: WebAssembly.Memory, thread_stack_size?: number }} module_or_path - Passing `InitInput` directly is deprecated.
* @param {WebAssembly.Memory} memory - Deprecated.
*
* @returns {Promise<InitOutput>}
*/
export default function __wbg_init (module_or_path?: { module_or_path: InitInput | Promise<InitInput>, memory?: WebAssembly.Memory, thread_stack_size?: number } | InitInput | Promise<InitInput>, memory?: WebAssembly.Memory): Promise<InitOutput>;
