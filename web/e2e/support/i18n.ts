import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const bundlePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../src/locales/en/translation.json",
);
const en: unknown = JSON.parse(readFileSync(bundlePath, "utf8"));

/**
 * Resolves an accessible name from the same `en` bundle the app renders under
 * `locale: "en-US"`, so specs never hardcode UI copy. Rewording a string keeps
 * tests passing; renaming a key fails them, which is the structural change that
 * should fail.
 */
export function t(key: string, vars: Record<string, string | number> = {}): string {
  const value = key
    .split(".")
    .reduce<unknown>((node, part) => (node as Record<string, unknown> | undefined)?.[part], en);

  if (typeof value !== "string") throw new Error(`missing i18n key: ${key}`);

  return value.replace(/{{(\w+)}}/g, (whole, name: string) =>
    name in vars ? String(vars[name]) : whole,
  );
}
