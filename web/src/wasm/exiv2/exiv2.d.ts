// dist/index.d.ts — Types for all entry points
export interface Exiv2Metadata {
  exif?: Record<string, unknown>;
  iptc?: Record<string, unknown>;
  xmp?: Record<string, unknown>;
}
export interface Exiv2Module {
  read(input: Uint8Array): Exiv2Metadata;
  readTagText(input: Uint8Array, key: string): string | undefined;
  readTagBytes(input: Uint8Array, key: string): Uint8Array | undefined;
  writeString(input: Uint8Array, key: string, value: string): Uint8Array;
  writeBytes(input: Uint8Array, key: string, value: Uint8Array): Uint8Array;
  locateFile?: (path: string) => string;
  calledRun?: boolean;
}
export interface CreateOptions {
  locateFile?: (path: string) => string;
  [k: string]: any;
}
export function createExiv2Module(
  options?: CreateOptions,
): Promise<Exiv2Module>;
export default createExiv2Module;

// UMD global
declare global {
  function createExiv2Module(options?: CreateOptions): Promise<Exiv2Module>;
}
