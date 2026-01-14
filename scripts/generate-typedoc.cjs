#!/usr/bin/env node

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const rootDir = path.resolve(__dirname, "..");
const webDir = path.join(rootDir, "web");
const docsRoot = path.join(rootDir, "docs");
const typedocOutDir = path.join(webDir, "docs");
const targetDocsDir = path.join(docsRoot, "en", "docs");
const targetSidebarFile = path.join(docsRoot, ".vitepress", "sidebar", "typedoc-sidebar.json");

function run(cmd, cwd) {
  console.log(`\n> ${cmd}${cwd ? ` (cwd: ${cwd})` : ""}`);
  execSync(cmd, { cwd, stdio: "inherit" });
}

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function removeIfExists(target) {
  fs.rmSync(target, { recursive: true, force: true });
}

function main() {
  if (!fs.existsSync(webDir)) {
    throw new Error(`Web directory not found at ${webDir}`);
  }
  ensureDir(path.dirname(targetSidebarFile));

  const webNodeModules = path.join(webDir, "node_modules");
  if (!fs.existsSync(webNodeModules)) {
    run("npm install --include=dev --prefer-offline --no-audit --progress=false", webDir);
  }

  run("npm run docs", webDir);

  const sidebarSrc = path.join(typedocOutDir, "typedoc-sidebar.json");
  if (!fs.existsSync(typedocOutDir)) {
    throw new Error("TypeDoc output directory not found. Check TypeDoc configuration and rerun.");
  }
  if (!fs.existsSync(sidebarSrc)) {
    throw new Error("typedoc-sidebar.json not found. Ensure typedoc-plugin-markdown theme output is enabled.");
  }

  // Move sidebar JSON into VitePress sidebar location
  removeIfExists(targetSidebarFile);
  fs.renameSync(sidebarSrc, targetSidebarFile);
  console.log(`Moved sidebar to ${path.relative(rootDir, targetSidebarFile)}`);

  // Replace docs/en/docs with freshly generated TypeDoc
  removeIfExists(targetDocsDir);
  ensureDir(path.dirname(targetDocsDir));
  fs.renameSync(typedocOutDir, targetDocsDir);
  console.log(`Updated TypeDoc at ${path.relative(rootDir, targetDocsDir)}`);

  console.log("\nTypeDoc generation complete.");
}

try {
  main();
} catch (err) {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
}
