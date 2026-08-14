/* tslint:disable */
/* eslint-disable */

/**
 * Streaming hasher for large files to maintain low memory usage.
 */
export class StreamingHasher {
    free(): void;
    [Symbol.dispose](): void;
    /**
     * Finalize the hash and return as a hex string.
     */
    finalize(): string;
    /**
     * Finalize the hash and return as raw bytes (32 bytes).
     */
    finalizeRaw(): Uint8Array;
    constructor();
    /**
     * Update the hasher with a chunk of data.
     */
    update(chunk: Uint8Array): void;
}

/**
 * Fast single-pass hashing for small buffers.
 */
export function hash_asset(buffer: Uint8Array): string;

/**
 * Verify if a buffer's hash matches the expected hex string.
 */
export function verify_asset_hash(buffer: Uint8Array, expected_hex: string): boolean;

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
    readonly memory: WebAssembly.Memory;
    readonly __wbg_streaminghasher_free: (a: number, b: number) => void;
    readonly hash_asset: (a: number, b: number) => [number, number];
    readonly streaminghasher_finalize: (a: number) => [number, number];
    readonly streaminghasher_finalizeRaw: (a: number) => [number, number];
    readonly streaminghasher_new: () => number;
    readonly streaminghasher_update: (a: number, b: number, c: number) => void;
    readonly verify_asset_hash: (a: number, b: number, c: number, d: number) => number;
    readonly __wbindgen_externrefs: WebAssembly.Table;
    readonly __wbindgen_malloc: (a: number, b: number) => number;
    readonly __wbindgen_free: (a: number, b: number, c: number) => void;
    readonly __wbindgen_realloc: (a: number, b: number, c: number, d: number) => number;
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
