#!/usr/bin/env node

// Extracts the Web half of the Lumilio Atlas: module ownership, the derived
// module import graph, rendered feature docs, the exported-symbol index that
// Atlas view anchors resolve against, and doc.ts diagram anchoring problems.
// The Go Atlas builder (server/tools/atlas) consumes this JSON; nothing here is
// checked in.

import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parseDocFile } from "@edwinzhancn/docts";
import { marked } from "marked";
import ts from "typescript";

const webRoot = fileURLToPath(new URL("../", import.meta.url));
const repoRoot = path.resolve(webRoot, "..");
const srcRoot = path.join(webRoot, "src");
const sourceExtensions = [".ts", ".tsx"];
const resolutionExtensions = [".d.ts", ".ts", ".tsx", ".js", ".jsx"];

// Lower-layer roots whose direct subdirectories are separate modules. Files
// placed directly in these roots belong to the root module itself.
const splitRoots = new Set(["features", "lib", "components"]);
const ignoredRoots = new Set(["assets", "locales", "styles", "shims"]);

type ModuleFacts = {
  id: string;
  kind: "feature" | "layer";
  layer: string;
  name: string;
  dir: string;
  files: number;
  lines: number;
  imports: string[];
  summary: string;
  docFile: string | null;
  docHtml: string;
};

type SymbolFacts = {
  name: string;
  module: string;
  file: string;
  line: number;
  kind: string;
  hash: string;
  members?: string[];
};

type Problem = { file: string; message: string };

function toPosix(value: string): string {
  return value.split(path.sep).join("/");
}

function repoRelative(filename: string): string {
  return toPosix(path.relative(repoRoot, filename));
}

function srcRelative(filename: string): string {
  return toPosix(path.relative(srcRoot, filename));
}

function isSource(filename: string): boolean {
  return (
    sourceExtensions.includes(path.extname(filename)) &&
    !filename.endsWith(".d.ts") &&
    !/\.(?:test|spec)\.[jt]sx?$/.test(filename) &&
    !/(^|\/)doc\.ts$/.test(toPosix(filename)) &&
    !toPosix(filename).includes("/__screenshots__/") &&
    !toPosix(filename).includes("/__mocks__/")
  );
}

function walk(directory: string, out: string[] = []): string[] {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) walk(filename, out);
    else if (entry.isFile() && isSource(filename)) out.push(filename);
  }
  return out;
}

function moduleDirOf(filename: string): string | null {
  const parts = srcRelative(filename).split("/");
  if (parts.length === 1) return parts[0] === "main.tsx" ? "web/src/main.tsx" : null;
  const [root, child] = parts;
  if (ignoredRoots.has(root)) return null;
  if (root === "lib" && child === "http-commons") return "web/src/lib/http-commons";
  if (splitRoots.has(root) && parts.length > 2) return `web/src/${root}/${child}`;
  return `web/src/${root}`;
}

function layerOf(moduleId: string): string {
  const root = moduleId.split("/")[2] ?? "";
  if (root === "main.tsx" || root === "app") return "app";
  if (root === "features") return "features";
  return root;
}

function resolveImport(importer: string, specifier: string): string | null {
  let candidate: string;
  if (specifier.startsWith("@/")) candidate = path.join(srcRoot, specifier.slice(2));
  else if (specifier.startsWith(".")) candidate = path.resolve(path.dirname(importer), specifier);
  else return null;

  const candidates = [
    candidate,
    ...resolutionExtensions.map((extension) => `${candidate}${extension}`),
    ...resolutionExtensions.map((extension) => path.join(candidate, `index${extension}`)),
  ];
  for (const filename of candidates) {
    try {
      if (statSync(filename).isFile()) return filename;
    } catch {
      // Try the next resolution candidate.
    }
  }
  return null;
}

function hash(text: string): string {
  return createHash("sha256").update(text).digest("hex").slice(0, 16);
}

function hasExportModifier(node: ts.Node): boolean {
  return Boolean(
    ts.canHaveModifiers(node) &&
    ts.getModifiers(node)?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword),
  );
}

function declarationKind(node: ts.Node): string {
  if (ts.isFunctionDeclaration(node)) return "function";
  if (ts.isClassDeclaration(node)) return "class";
  if (ts.isInterfaceDeclaration(node)) return "interface";
  if (ts.isTypeAliasDeclaration(node)) return "type";
  if (ts.isEnumDeclaration(node)) return "enum";
  return "const";
}

// Enumerable members let lifecycle views prove they draw every state: enum
// member names, or the literals of a string-literal union type alias.
function enumMembers(node: ts.Node): string[] {
  if (ts.isEnumDeclaration(node)) {
    return node.members.map((member) => member.name.getText()).sort();
  }
  if (ts.isTypeAliasDeclaration(node) && ts.isUnionTypeNode(node.type)) {
    const literals = node.type.types.map((member) =>
      ts.isLiteralTypeNode(member) && ts.isStringLiteral(member.literal)
        ? member.literal.text
        : null,
    );
    if (literals.every((literal): literal is string => literal !== null)) return literals.sort();
  }
  return [];
}

function collectSymbols(
  sourceFile: ts.SourceFile,
  moduleId: string,
  file: string,
  out: SymbolFacts[],
): void {
  const push = (name: string, node: ts.Node) => {
    const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
    const members = enumMembers(node);
    out.push({
      name,
      module: moduleId,
      file,
      line: line + 1,
      kind: declarationKind(node),
      hash: hash(node.getText(sourceFile)),
      ...(members.length > 0 ? { members } : {}),
    });
  };

  for (const statement of sourceFile.statements) {
    if (!hasExportModifier(statement)) continue;
    if (
      (ts.isFunctionDeclaration(statement) ||
        ts.isClassDeclaration(statement) ||
        ts.isInterfaceDeclaration(statement) ||
        ts.isTypeAliasDeclaration(statement) ||
        ts.isEnumDeclaration(statement)) &&
      statement.name
    ) {
      push(statement.name.text, statement);
    } else if (ts.isVariableStatement(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        if (ts.isIdentifier(declaration.name)) push(declaration.name.text, statement);
      }
    }
  }
}

function readImportSpecifiers(sourceFile: ts.SourceFile): string[] {
  const specifiers: string[] = [];
  const visit = (node: ts.Node) => {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      specifiers.push(node.moduleSpecifier.text);
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments.length === 1 &&
      ts.isStringLiteral(node.arguments[0])
    ) {
      specifiers.push(node.arguments[0].text);
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return specifiers;
}

// A Mermaid node label that reads like a code identifier (camelCase, PascalCase
// with an inner capital, or a use* hook) claims to name real code, so the doc.ts
// must import it. Plain-language labels stay free prose.
const mermaidNodePattern =
  /\b([A-Za-z_][\w]*)\s*(?:\[\[|\[\(|\(\[|\(\(|\[\/|\[\\|\[|\(|\{\{|\{|>)\s*"?([^"\])}]+?)"?\s*(?:\]\]|\)\]|\]\)|\)\)|\/\]|\\\]|\]|\)|\}\}|\})/g;

function identifierLike(label: string): string | null {
  const trimmed = label.trim().replace(/\(\)$/, "");
  if (!/^[A-Za-z_$][\w$]*$/.test(trimmed)) return null;
  if (/[a-z0-9][A-Z]/.test(trimmed) || /^use[A-Z]/.test(trimmed)) return trimmed;
  return null;
}

function checkDocDiagrams(docFile: string, imports: Map<string, string>, problems: Problem[]) {
  const text = readFileSync(docFile, "utf8");
  const comment = text
    .split("\n")
    .map((line) => line.replace(/^\s*\/?\*+\/?\s?/, ""))
    .join("\n");
  const blocks = comment.match(/```mermaid\n([\s\S]*?)```/g) ?? [];
  for (const block of blocks) {
    for (const match of block.matchAll(mermaidNodePattern)) {
      const identifier = identifierLike(match[2]);
      if (identifier && !imports.has(identifier)) {
        problems.push({
          file: repoRelative(docFile),
          message: `Mermaid node "${match[1]}" is labelled with code identifier "${identifier}" but doc.ts does not import it; add an \`import type\` or relabel it in plain language`,
        });
      }
    }
  }
}

function renderFeatureDoc(featureDir: string): { html: string; summary: string } {
  const markdown = readFileSync(path.join(featureDir, "doc.md"), "utf8");
  const dirRel = repoRelative(featureDir);
  const html = (marked.parse(markdown, { async: false }) as string).replace(
    /href="\.\/([^"]+)"/g,
    (_match, target: string) =>
      `href="#/source/${dirRel}/${target}" data-source="${dirRel}/${target}"`,
  );
  const paragraphs = markdown
    .split(/\n\s*\n/)
    .map((paragraph) => paragraph.trim())
    .filter((paragraph) => paragraph && !paragraph.startsWith("#") && !paragraph.startsWith("```"));
  const summary = (paragraphs[0] ?? "")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/\s+/g, " ");
  return { html, summary };
}

const files = walk(srcRoot);
const modules = new Map<string, ModuleFacts>();
const symbols: SymbolFacts[] = [];
const problems: Problem[] = [];

for (const filename of files) {
  const moduleId = moduleDirOf(filename);
  if (!moduleId) continue;
  const text = readFileSync(filename, "utf8");
  let module = modules.get(moduleId);
  if (!module) {
    const root = moduleId.split("/")[2];
    module = {
      id: moduleId,
      kind: root === "features" ? "feature" : "layer",
      layer: layerOf(moduleId),
      name: moduleId.split("/").slice(2).join("/"),
      dir: moduleId,
      files: 0,
      lines: 0,
      imports: [],
      summary: "",
      docFile: null,
      docHtml: "",
    };
    modules.set(moduleId, module);
  }
  module.files += 1;
  module.lines += text.split("\n").length;

  const sourceFile = ts.createSourceFile(
    filename,
    text,
    ts.ScriptTarget.Latest,
    true,
    filename.endsWith("x") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  collectSymbols(sourceFile, moduleId, repoRelative(filename), symbols);

  const imports = new Set(module.imports);
  for (const specifier of readImportSpecifiers(sourceFile)) {
    const resolved = resolveImport(filename, specifier);
    if (!resolved) continue;
    const target = moduleDirOf(resolved);
    if (target && target !== moduleId) imports.add(target);
  }
  module.imports = [...imports];
}

for (const module of modules.values()) {
  module.imports = module.imports.filter((target) => modules.has(target)).sort();
  if (module.kind !== "feature") continue;
  const featureDir = path.join(repoRoot, module.dir);
  const docTs = path.join(featureDir, "doc.ts");
  try {
    statSync(docTs);
  } catch {
    problems.push({ file: module.dir, message: "feature has no doc.ts" });
    continue;
  }
  module.docFile = repoRelative(docTs);
  const parsed = parseDocFile(docTs);
  checkDocDiagrams(docTs, parsed.imports, problems);
  const rendered = renderFeatureDoc(featureDir);
  module.docHtml = rendered.html;
  module.summary = rendered.summary;
}

const output = {
  modules: [...modules.values()].sort((a, b) => a.id.localeCompare(b.id)),
  symbols: symbols.sort((a, b) => a.file.localeCompare(b.file) || a.line - b.line),
  problems,
};

const outFlag = process.argv.indexOf("--out");
const json = `${JSON.stringify(output)}\n`;
if (outFlag !== -1 && process.argv[outFlag + 1]) writeFileSync(process.argv[outFlag + 1], json);
else process.stdout.write(json);
