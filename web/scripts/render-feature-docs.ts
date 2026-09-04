import { existsSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { parseDocFile, renderMarkdown } from "@edwinzhancn/docts";

const featuresDir = path.resolve("src/features");

let rendered = 0;
for (const name of readdirSync(featuresDir)) {
  const docTs = path.join(featuresDir, name, "doc.ts");
  if (!existsSync(docTs)) continue;
  writeFileSync(docTs.replace(/\.ts$/, ".md"), renderMarkdown(parseDocFile(docTs)));
  rendered += 1;
}

if (rendered === 0) {
  throw new Error(`no feature doc.ts files found under ${featuresDir}`);
}
