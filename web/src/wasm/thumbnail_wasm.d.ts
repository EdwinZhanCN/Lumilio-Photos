/* tslint:disable */
/* eslint-disable */

/**
 * Declares the default init function, which your code calls as "init()".
 */
export default function init(): Promise<void>;

/**
 * Declares the generate_thumbnail function from the WASM.
 */
export function generate_thumbnail(buffer: Uint8Array, max_size: number): Uint8Array;

/**
 * Any other classes or types remain as before.
 */
export class ThumbnailResult {
  private constructor();
  free(): void;
  readonly width: number;
  readonly height: number;
  readonly data: Uint8Array;
}