import "@/lib/i18n";
import i18n from "i18next";

/**
 * Resolve an app translation key to its current `en` copy through the same
 * i18next instance the app renders with. Specs match on keys, not literals:
 * rewording a string keeps them green, renaming or removing a key fails them —
 * the structural change that should fail. Mirrors `e2e/support/i18n.ts` for the
 * in-process integration project. Never hardcode app copy in a spec; only
 * strings the spec itself owns (route sentinels, fixtures) may be literals.
 */
export function t(key: string, options?: Record<string, unknown>): string {
  return i18n.getFixedT("en")(key, options);
}
