import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import openapiTS, { astToString } from "openapi-typescript";
import { parse } from "yaml";

const inputPath = resolve(process.cwd(), "../server/docs/swagger.yaml");
const outputPath = resolve(process.cwd(), "src/lib/http-commons/schema.d.ts");
const document = parse(await readFile(inputPath, "utf8"));

const HTTP_METHODS = ["get", "put", "post", "delete", "options", "head", "patch", "trace"];

function isGeneratedEmptyObject(schema) {
  if (!schema || typeof schema !== "object" || Array.isArray(schema)) return false;
  const keys = Object.keys(schema);
  return schema.type === "object" && keys.every((key) => key === "type");
}

function normalizeRequiredJsonBodies(openapi) {
  let normalized = 0;

  for (const pathItem of Object.values(openapi.paths ?? {})) {
    for (const method of HTTP_METHODS) {
      const operation = pathItem?.[method];
      const requestBody = operation?.requestBody;
      if (!requestBody || requestBody.required !== true) continue;

      const media = requestBody.content?.["application/json"];
      const alternatives = media?.schema?.oneOf;
      if (!Array.isArray(alternatives)) continue;

      const meaningful = alternatives.filter((schema) => !isGeneratedEmptyObject(schema));
      if (meaningful.length === alternatives.length) continue;
      if (meaningful.length !== 1) {
        throw new Error(
          `Cannot safely normalize required JSON body for ${method.toUpperCase()}: expected one non-empty schema`,
        );
      }

      media.schema = meaningful[0];
      normalized += 1;
    }
  }

  return normalized;
}

const normalizedCount = normalizeRequiredJsonBodies(document);
const nodes = await openapiTS(document);
await writeFile(outputPath, astToString(nodes));
console.log(`Generated ${outputPath}; normalized ${normalizedCount} required JSON request bodies.`);
