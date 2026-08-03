import { readFile } from "node:fs/promises";
import { gzipSync } from "node:zlib";
import { fileURLToPath } from "node:url";
import path from "node:path";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const indexHtml = await readFile(path.join(webRoot, "dist/index.html"), "utf8");
const entryMatch = indexHtml.match(/<script[^>]+type="module"[^>]+src="([^"]+\.js)"/);
if (!entryMatch) throw new Error("Unable to locate the production entry chunk in dist/index.html");

const entryPath = path.join(webRoot, "dist", entryMatch[1].replace(/^\//, ""));
const entry = await readFile(entryPath);
const gzipBytes = gzipSync(entry).byteLength;
const budgetBytes = 420 * 1024;

if (gzipBytes > budgetBytes) {
  throw new Error(
    `Entry chunk ${path.basename(entryPath)} is ${(gzipBytes / 1024).toFixed(1)} KiB gzip; budget is ${budgetBytes / 1024} KiB`,
  );
}

console.log(
  `Entry chunk budget passed: ${path.basename(entryPath)} ${(gzipBytes / 1024).toFixed(1)} KiB gzip <= ${budgetBytes / 1024} KiB`,
);
