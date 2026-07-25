/** Browser-only replacement for an unreachable Node fallback in ExifTool WASM. */
export async function readFile(): Promise<never> {
  throw new Error("node:fs/promises is unavailable in the browser");
}
