import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "../locales/en/translation.json";
import zh from "../locales/zh/translation.json";

// Desktop locale is a host preference persisted in settings.v1.json — there is
// no browser-language detection; applyLocale is driven by the snapshot.
const resources = {
  en: { translation: en },
  zh: { translation: zh },
} as const;

let initialized = false;

export function initI18n(initialLocale: string) {
  if (initialized) return;
  initialized = true;
  // Resources are bundled synchronously. Finish initialization before the
  // first React render so the control panel never briefly renders translation
  // keys while the host snapshot is still arriving.
  void i18n.use(initReactI18next).init({
    resources,
    lng: normalizeLocale(initialLocale),
    fallbackLng: "en",
    supportedLngs: ["en", "zh"],
    interpolation: { escapeValue: false },
    initAsync: false,
  });
}

export function applyLocale(locale: string) {
  if (!initialized) {
    initI18n(locale);
    return;
  }
  const next = normalizeLocale(locale);
  if (i18n.resolvedLanguage !== next) {
    void i18n.changeLanguage(next);
  }
}

// "zh-CN" resolves to the "zh" bundle; unknown values fall back to "en".
function normalizeLocale(locale: string): string {
  if (locale.trim().toLowerCase().startsWith("zh")) return "zh";
  return "en";
}
