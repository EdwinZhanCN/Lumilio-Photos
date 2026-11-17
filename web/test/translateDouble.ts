import type { TFunction } from "i18next";

export type TranslateCall = {
  key: string;
  options?: string | Record<string, unknown>;
};

/**
 * Unit-level translation double.
 *
 * Unit specs characterize *which key* a rule selects; the copy itself belongs to
 * `src/locales` and is covered by `lumilio-frontend-i18n` extraction. The stub
 * echoes the key and records every call, so a spec never embeds app copy and a
 * renamed key fails the mapping immediately.
 *
 * The cast is confined to this double: i18next's `TFunction` overloads cannot be
 * satisfied structurally without restating the whole signature.
 */
export function translateDouble(): { t: TFunction; calls: TranslateCall[] } {
  const calls: TranslateCall[] = [];
  const t = (key: string, options?: string | Record<string, unknown>) => {
    calls.push({ key, options });
    return key;
  };
  return { t: t as unknown as TFunction, calls };
}
