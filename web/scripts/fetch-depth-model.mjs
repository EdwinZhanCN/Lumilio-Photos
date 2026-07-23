// Vendor the Depth Anything V2 (small, q4f16) model and the onnxruntime-web wasm
// into public/ so the Studio depth feature runs fully self-hosted — no runtime
// dependency on the HuggingFace CDN. The downloaded assets are gitignored; run
// this once on setup (wired into `make setup`), like `playwright install`.

import path from "node:path";
import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, "..");

const MODEL_ID = "onnx-community/depth-anything-v2-small-ONNX";
const BASE = `https://huggingface.co/${MODEL_ID}/resolve/main`;
const MODEL_FILES = [
  "config.json",
  "preprocessor_config.json",
  "onnx/model_q4f16.onnx",
  "onnx/model_q4f16.onnx_data",
];

const modelDir = path.join(webRoot, "public", "models", MODEL_ID);
const ortDir = path.join(webRoot, "public", "ort");

async function exists(p) {
  return fs.access(p).then(
    () => true,
    () => false,
  );
}

async function download(file) {
  const dest = path.join(modelDir, file);
  if (await exists(dest)) {
    console.log(`✓ ${file} (cached)`);
    return;
  }
  await fs.mkdir(path.dirname(dest), { recursive: true });
  process.stdout.write(`↓ ${file} … `);
  const res = await fetch(`${BASE}/${file}`);
  if (!res.ok) throw new Error(`${file}: HTTP ${res.status}`);
  const buf = Buffer.from(await res.arrayBuffer());
  await fs.writeFile(dest, buf);
  console.log(`${(buf.length / 1e6).toFixed(1)} MB`);
}

async function findOrtDist() {
  // onnxruntime-web ships under transformers in the pnpm store; locate the dist
  // that actually contains the wasm binaries.
  const pnpm = path.join(webRoot, "node_modules", ".pnpm");
  const entries = await fs.readdir(pnpm);
  const candidates = entries
    .filter((e) => e.startsWith("onnxruntime-web@") || e.startsWith("@huggingface+transformers@"))
    .map((e) => path.join(pnpm, e, "node_modules", "onnxruntime-web", "dist"));
  for (const dist of candidates) {
    if (!(await exists(dist))) continue;
    if ((await fs.readdir(dist)).some((f) => f.endsWith(".wasm"))) return dist;
  }
  throw new Error("onnxruntime-web dist with wasm not found");
}

async function copyOrtWasm() {
  const dist = await findOrtDist();
  // Both the .wasm binaries AND their .mjs loader modules — onnxruntime-web
  // dynamically imports the .mjs from wasmPaths at runtime, so serving only the
  // .wasm yields "Importing a module script failed" for the WebGPU backend.
  const files = (await fs.readdir(dist)).filter((f) => /^ort-wasm-.*\.(wasm|mjs)$/.test(f));
  await fs.mkdir(ortDir, { recursive: true });
  for (const file of files) {
    await fs.copyFile(path.join(dist, file), path.join(ortDir, file));
  }
  console.log(`✓ ort runtime ×${files.length} (wasm + mjs)`);
}

async function main() {
  console.log(`Vendoring depth model into public/ …`);
  for (const file of MODEL_FILES) await download(file);
  await copyOrtWasm();
  console.log("Done.");
}

main().catch((error) => {
  console.error(`Failed: ${error.message}`);
  process.exit(1);
});
