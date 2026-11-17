import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const localesRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../src/locales");
const en: unknown = JSON.parse(readFileSync(path.join(localesRoot, "en/translation.json"), "utf8"));
const zh: unknown = JSON.parse(readFileSync(path.join(localesRoot, "zh/translation.json"), "utf8"));

function resolve(bundle: unknown, key: string, vars: Record<string, string | number>): string {
  const value = key
    .split(".")
    .reduce<unknown>((node, part) => (node as Record<string, unknown> | undefined)?.[part], bundle);

  if (typeof value !== "string") throw new Error(`missing i18n key: ${key}`);

  return value.replace(/{{(\w+)}}/g, (whole, name: string) =>
    name in vars ? String(vars[name]) : whole,
  );
}

/**
 * Resolves an accessible name from the same `en` bundle the app renders under
 * `locale: "en-US"`, so specs never hardcode UI copy. Rewording a string keeps
 * tests passing; renaming a key fails them, which is the structural change that
 * should fail.
 */
export function t(key: string, vars: Record<string, string | number> = {}): string {
  return resolve(en, key, vars);
}

/**
 * The same key resolved from the `zh` bundle, for the one spec that switches
 * the interface language and must read what the app renders afterwards.
 */
export function tz(key: string, vars: Record<string, string | number> = {}): string {
  return resolve(zh, key, vars);
}
