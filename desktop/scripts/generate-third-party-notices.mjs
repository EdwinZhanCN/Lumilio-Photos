import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, realpathSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const output = join(root, "desktop/licenses/THIRD_PARTY_NOTICES.txt");
const licenseNames = /^(license|licence|copying|notice)(\..*)?$/i;
const entries = new Map();

function licenseFiles(dir) {
  if (!dir || !existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((name) => licenseNames.test(name) && statSync(join(dir, name)).isFile())
    .sort()
    .map((name) => ({ name, text: readFileSync(join(dir, name), "utf8").trim() }));
}

function add(key, title, source, files, declared = "") {
  entries.set(key, { title, source, declared, files });
}

for (const moduleDir of [join(root, "desktop"), join(root, "server")]) {
  const packages = JSON.parse(`[${execFileSync("go", ["list", "-deps", "-json", "./..."], { cwd: moduleDir, encoding: "utf8", maxBuffer: 64 * 1024 * 1024 }).trim().replace(/}\s*{/g, "},{")}]`);
  const modules = [...new Map(packages.filter((pkg) => pkg.Module).map((pkg) => [`${pkg.Module.Path}@${pkg.Module.Version}`, pkg.Module])).values()];
  for (const mod of modules) {
    if (mod.Main || mod.Path === "server") continue;
    const actual = mod.Replace || mod;
    if (!actual.Dir) continue;
    add(`go:${mod.Path}@${mod.Version || actual.Version || "local"}`, `${mod.Path} ${mod.Version || actual.Version || ""}`.trim(), mod.Path, licenseFiles(actual.Dir));
  }
}

const nodeModules = join(root, "web/node_modules");
if (!existsSync(nodeModules)) throw new Error("web/node_modules is missing; run `cd web && vp install`");
function visit(dir) {
  for (const item of readdirSync(dir, { withFileTypes: true })) {
    if (item.name.startsWith(".")) continue;
    const path = join(dir, item.name);
    if (item.isDirectory() && item.name.startsWith("@")) visit(path);
    else if (item.isDirectory() || item.isSymbolicLink()) {
      const packageDir = realpathSync(path);
      const manifestPath = join(packageDir, "package.json");
      if (!existsSync(manifestPath)) continue;
      const pkg = JSON.parse(readFileSync(manifestPath, "utf8"));
      if (!pkg.name || pkg.private) continue;
      const source = typeof pkg.repository === "string" ? pkg.repository : pkg.repository?.url || pkg.homepage || `https://www.npmjs.com/package/${pkg.name}`;
      add(`npm:${pkg.name}@${pkg.version}`, `${pkg.name} ${pkg.version}`, source, licenseFiles(packageDir), typeof pkg.license === "string" ? pkg.license : "");
    }
  }
}
visit(nodeModules);

const lines = [
  "THIRD-PARTY SOFTWARE NOTICES",
  "============================",
  "",
  "Lumilio Photos incorporates the following third-party software. This file is generated; do not edit it manually.",
  "",
];
for (const entry of [...entries.values()].sort((a, b) => a.title.localeCompare(b.title))) {
  lines.push("--------------------------------------------------------------------------------", entry.title, entry.declared ? `Declared license: ${entry.declared}` : "", `Source: ${entry.source}`, "");
  if (!entry.files.length) lines.push("No license text was present in the distributed package metadata; consult the source link above.", "");
  for (const file of entry.files) lines.push(`[${basename(file.name)}]`, file.text, "");
}
writeFileSync(output, `${lines.filter((line) => line !== undefined).join("\n")}\n`);
console.log(`Wrote ${output} with ${entries.size} dependency entries.`);
