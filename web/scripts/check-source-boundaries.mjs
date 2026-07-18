#!/usr/bin/env node

import { readdir, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const srcRoot = fileURLToPath(new URL("../src/", import.meta.url));
const sourceExtensions = [".ts", ".tsx", ".mts", ".cts", ".js", ".jsx"];
const resolutionExtensions = [".d.ts", ...sourceExtensions];
const indexFiles = resolutionExtensions.map((extension) => `index${extension}`);
const lowerLayerRoots = new Set([
  "components",
  "config",
  "contexts",
  "hooks",
  "lib",
  "types",
  "workers",
]);

// Purpose-built feature entry points stay narrow. Everything else crosses a
// feature through `@/features/<feature>`.
const allowedFeatureEntries = new Set(["assets/map", "assets/picker"]);

const standardFeatureDirectories = new Set([
  "api",
  "components",
  "docs",
  "hooks",
  "modules",
  "routes",
  "state",
  "utils",
]);
const standardFeatureRootFiles = new Set(["doc.md", "doc.ts", "index.ts", "types.ts"]);
const featureDirectoryExceptions = new Map([["assets", new Set(["map", "picker"])]]);

// Only the application entry point may enter app composition from the source
// root. The production smoke entry composes public feature APIs directly.
const appImportEntrypoints = new Set(["main.tsx"]);

// Worker entry points are lower-layer files but may register specific
// feature-owned, worker-safe runners. Keep this list exact and review additions.
const allowedLowerLayerFeatureImports = new Map([
  [
    "workers/tool.worker.ts",
    new Set([
      "@/features/studio/modules/tools/border/borderRunner",
      "@/features/studio/modules/tools/types",
    ]),
  ],
]);

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function relativeToSrc(filename) {
  return toPosix(path.relative(srcRoot, filename));
}

function shouldCheckFile(filename) {
  const relative = relativeToSrc(filename);
  return (
    sourceExtensions.includes(path.extname(filename)) &&
    !/(^|\/)doc\.ts$/.test(relative) &&
    !relative.startsWith("wasm/") &&
    relative !== "vite-env.d.ts" &&
    relative !== "lib/http-commons/schema.d.ts"
  );
}

function isTestFile(filename) {
  return /\.(?:test|spec)\.[cm]?[jt]sx?$/.test(relativeToSrc(filename));
}

async function collectSourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectSourceFiles(filename)));
    } else if (entry.isFile() && shouldCheckFile(filename)) {
      files.push(filename);
    }
  }

  return files;
}

async function checkFeatureRootShape(violations) {
  const featuresRoot = path.join(srcRoot, "features");
  const features = await readdir(featuresRoot, { withFileTypes: true });

  for (const feature of features) {
    if (!feature.isDirectory()) continue;

    const entries = await readdir(path.join(featuresRoot, feature.name), {
      withFileTypes: true,
    });
    const entryNames = new Set(entries.map((entry) => entry.name));
    const allowedDirectories = featureDirectoryExceptions.get(feature.name) ?? new Set();

    for (const entry of entries) {
      if (
        entry.isDirectory() &&
        !standardFeatureDirectories.has(entry.name) &&
        !allowedDirectories.has(entry.name)
      ) {
        violations.push(`features/${feature.name}: non-standard root directory '${entry.name}'`);
      }

      if (entry.isFile() && !standardFeatureRootFiles.has(entry.name)) {
        violations.push(`features/${feature.name}: non-standard root file '${entry.name}'`);
      }
    }

    if (entryNames.has("doc.ts") !== entryNames.has("doc.md")) {
      violations.push(`features/${feature.name}: doc.ts and generated doc.md must exist together`);
    }
  }
}

function featureOf(filename) {
  const parts = relativeToSrc(filename).split("/");
  return parts[0] === "features" && parts[1] ? parts[1] : null;
}

function rootLayerOf(filename) {
  return relativeToSrc(filename).split("/")[0] ?? "";
}

function isAllowedLowerLayerFeatureImport(importerRelative, specifier) {
  return allowedLowerLayerFeatureImports.get(importerRelative)?.has(specifier) ?? false;
}

function importClauseIsTypeOnly(clause) {
  if (!clause) return false;
  if (clause.isTypeOnly) return true;
  if (clause.name) return false;

  const bindings = clause.namedBindings;
  return (
    bindings &&
    ts.isNamedImports(bindings) &&
    bindings.elements.length > 0 &&
    bindings.elements.every((element) => element.isTypeOnly)
  );
}

function exportClauseIsTypeOnly(node) {
  if (node.isTypeOnly) return true;
  return (
    node.exportClause &&
    ts.isNamedExports(node.exportClause) &&
    node.exportClause.elements.length > 0 &&
    node.exportClause.elements.every((element) => element.isTypeOnly)
  );
}

function readImports(filename, sourceText) {
  const sourceFile = ts.createSourceFile(
    filename,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    filename.endsWith("x") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const imports = [];

  function visit(node) {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      imports.push({
        specifier: node.moduleSpecifier.text,
        typeOnly: importClauseIsTypeOnly(node.importClause),
      });
    } else if (
      ts.isExportDeclaration(node) &&
      node.moduleSpecifier &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      imports.push({
        specifier: node.moduleSpecifier.text,
        typeOnly: exportClauseIsTypeOnly(node),
      });
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments.length === 1 &&
      ts.isStringLiteral(node.arguments[0])
    ) {
      imports.push({ specifier: node.arguments[0].text, typeOnly: false });
    }

    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
  return imports;
}

async function isFile(filename) {
  try {
    return (await stat(filename)).isFile();
  } catch {
    return false;
  }
}

async function resolveInternalImport(importer, specifier) {
  let candidate;
  if (specifier.startsWith("@/")) {
    candidate = path.join(srcRoot, specifier.slice(2));
  } else if (specifier.startsWith(".")) {
    candidate = path.resolve(path.dirname(importer), specifier);
  } else {
    return null;
  }

  if (await isFile(candidate)) return candidate;

  const hasSourceExtension = resolutionExtensions.some((extension) =>
    candidate.endsWith(extension),
  );
  if (!hasSourceExtension) {
    for (const extension of resolutionExtensions) {
      const filename = `${candidate}${extension}`;
      if (await isFile(filename)) return filename;
    }
  }

  for (const indexFile of indexFiles) {
    const filename = path.join(candidate, indexFile);
    if (await isFile(filename)) return filename;
  }

  return undefined;
}

function isPublicFeatureImport(specifier, feature) {
  if (specifier === `@/features/${feature}`) return true;
  const featureEntry = specifier.slice("@/features/".length);
  return allowedFeatureEntries.has(featureEntry);
}

function stronglyConnectedComponents(graph) {
  let index = 0;
  const indices = new Map();
  const lowLinks = new Map();
  const stack = [];
  const onStack = new Set();
  const components = [];

  function connect(node) {
    indices.set(node, index);
    lowLinks.set(node, index);
    index += 1;
    stack.push(node);
    onStack.add(node);

    for (const neighbor of graph.get(node) ?? []) {
      if (!indices.has(neighbor)) {
        connect(neighbor);
        lowLinks.set(node, Math.min(lowLinks.get(node), lowLinks.get(neighbor)));
      } else if (onStack.has(neighbor)) {
        lowLinks.set(node, Math.min(lowLinks.get(node), indices.get(neighbor)));
      }
    }

    if (lowLinks.get(node) !== indices.get(node)) return;

    const component = [];
    let member;
    do {
      member = stack.pop();
      onStack.delete(member);
      component.push(member);
    } while (member !== node);
    components.push(component);
  }

  for (const node of graph.keys()) {
    if (!indices.has(node)) connect(node);
  }

  return components;
}

const files = await collectSourceFiles(srcRoot);
const runtimeFiles = files.filter((filename) => !isTestFile(filename));
const runtimeFileSet = new Set(runtimeFiles.map((filename) => path.resolve(filename)));
const graph = new Map(runtimeFiles.map((filename) => [path.resolve(filename), new Set()]));
const featureNames = new Set(runtimeFiles.map(featureOf).filter(Boolean));
const featureGraph = new Map([...featureNames].map((feature) => [feature, new Set()]));
const violations = [];
let internalRuntimeEdges = 0;

await checkFeatureRootShape(violations);

for (const filename of files) {
  const sourceText = ts.sys.readFile(filename) ?? "";
  const importer = path.resolve(filename);
  const importerRelative = relativeToSrc(importer);
  const importerFeature = featureOf(importer);
  const importerRoot = rootLayerOf(importer);
  const importerIsTest = isTestFile(importer);
  const importerParts = importerRelative.split("/");
  const featureSection = importerFeature ? importerParts[2] : null;
  const basename = path.basename(importerRelative);

  if (
    importerFeature &&
    featureSection !== "state" &&
    /(?:Provider|Store|store|Reducer|reducer|Context|context)\.[cm]?[jt]sx?$/.test(basename)
  ) {
    violations.push(
      `${importerRelative}: shared feature state modules belong in the feature state directory`,
    );
  }

  if (
    importerFeature &&
    !importerIsTest &&
    featureSection !== "state" &&
    /\b(?:localStorage|sessionStorage)\b/.test(sourceText)
  ) {
    violations.push(
      `${importerRelative}: persisted feature state belongs in the feature state directory`,
    );
  }

  if (
    importerFeature &&
    featureSection === "hooks" &&
    /\$api\.use(?:Infinite)?Query\s*\(/.test(sourceText)
  ) {
    violations.push(
      `${importerRelative}: server-state query definitions belong in the feature api directory`,
    );
  }

  for (const imported of readImports(filename, sourceText)) {
    const { specifier, typeOnly } = imported;
    if (!specifier.startsWith("@/") && !specifier.startsWith(".")) continue;

    const resolved = await resolveInternalImport(importer, specifier);
    if (resolved === undefined) {
      violations.push(`${importerRelative}: unresolved internal import '${specifier}'`);
      continue;
    }
    if (resolved === null) continue;

    const resolvedAbsolute = path.resolve(resolved);
    const importedFeature = featureOf(resolvedAbsolute);
    const importedRoot = rootLayerOf(resolvedAbsolute);

    if (
      importedRoot === "app" &&
      importerRoot !== "app" &&
      !appImportEntrypoints.has(importerRelative)
    ) {
      violations.push(`${importerRelative}: only composition entry points may depend on app`);
    }

    if (!importerIsTest && isTestFile(resolvedAbsolute)) {
      violations.push(
        `${importerRelative}: runtime source cannot import test module '${specifier}'`,
      );
    }

    if (specifier.startsWith("@/features/") && importerFeature === importedFeature) {
      violations.push(
        `${importerRelative}: use a relative import inside feature '${importerFeature}' (${specifier})`,
      );
    }

    if (
      importerFeature &&
      importedFeature &&
      importerFeature !== importedFeature &&
      !isPublicFeatureImport(specifier, importedFeature)
    ) {
      violations.push(
        `${importerRelative}: cross feature '${importedFeature}' through its public entry (${specifier})`,
      );
    }

    if (
      lowerLayerRoots.has(importerRoot) &&
      importedFeature &&
      !isAllowedLowerLayerFeatureImport(importerRelative, specifier)
    ) {
      violations.push(
        `${importerRelative}: lower layer '${importerRoot}' cannot depend on feature '${importedFeature}'`,
      );
    }

    if (!importerIsTest && !typeOnly && runtimeFileSet.has(resolvedAbsolute)) {
      graph.get(importer).add(resolvedAbsolute);
      internalRuntimeEdges += 1;
    }

    if (
      !typeOnly &&
      !importerIsTest &&
      importerFeature &&
      importedFeature &&
      importerFeature !== importedFeature
    ) {
      featureGraph.get(importerFeature).add(importedFeature);
    }
  }
}

const cycles = stronglyConnectedComponents(graph)
  .filter((component) => component.length > 1 || graph.get(component[0])?.has(component[0]))
  .map((component) => component.map(relativeToSrc).sort());

for (const cycle of cycles) {
  violations.push(`runtime import cycle: ${cycle.join(" -> ")}`);
}

const featureCycles = stronglyConnectedComponents(featureGraph)
  .filter((component) => component.length > 1)
  .map((component) => component.sort());

for (const cycle of featureCycles) {
  violations.push(`feature dependency cycle: ${cycle.join(" -> ")}`);
}

if (violations.length > 0) {
  console.error(`Source boundary check failed (${violations.length} violations):`);
  for (const violation of violations.sort()) {
    console.error(`- ${violation}`);
  }
  process.exitCode = 1;
} else {
  console.log(
    `Source boundaries passed: ${runtimeFiles.length} runtime modules, ${files.length - runtimeFiles.length} test modules, ${internalRuntimeEdges} runtime edges, 0 cycles.`,
  );
}
