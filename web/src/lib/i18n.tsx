import React, { PropsWithChildren, Suspense } from "react";
import i18n, { i18n as I18NextInstance } from "i18next";
import {
  I18nextProvider,
  initReactI18next,
  useTranslation,
} from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import HttpBackend from "i18next-http-backend";

export const SUPPORTED_LANGUAGES = ["en", "zh"] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

function initI18n(instance: I18NextInstance) {
  if (instance.isInitialized) return instance;

  instance
    .use(HttpBackend)
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
      // Enable debug in dev only
      debug:
        typeof import.meta !== "undefined"
          ? Boolean(import.meta.env?.DEV)
          : false,

      // Fallback and supported languages
      fallbackLng: "en",
      supportedLngs: SUPPORTED_LANGUAGES as unknown as string[],

      // Namespaces (single default namespace "translation")
      ns: ["translation"],
      defaultNS: "translation",

      // Only load language part (e.g., "en-US" -> "en")
      load: "languageOnly",

      // Backend configuration: serve from /public/locales
      backend: {
        loadPath: "/locales/{{lng}}/translation.json",
      },

      // Language detection strategy
      detection: {
        order: [
          "querystring",
          "localStorage",
          "navigator",
          "htmlTag",
          "cookie",
        ],
        lookupQuerystring: "lng",
        caches: ["localStorage"],
      },

      // React i18next config
      react: {
        useSuspense: true,
      },

      // Never escape because React already handles XSS
      interpolation: {
        escapeValue: false,
      },
    })
    .catch((err: unknown) => {
      // Non-fatal: app can still render, just log the error
      console.error("[i18n] initialization failed:", err);
    });

  return instance;
}

// Initialize once at module load
initI18n(i18n);

/**
 * I18nProvider wraps the app with I18nextProvider and Suspense for async resource loading.
 */
export function I18nProvider({ children }: PropsWithChildren): React.ReactNode {
  return (
    <I18nextProvider i18n={i18n}>
      <Suspense fallback={null}>{children}</Suspense>
    </I18nextProvider>
  );
}

/**
 * Convenience hook re-export, so consumers can import from a single module.
 * Example:
 *   const { t } = useI18n();
 */
export function useI18n(ns?: string | string[]) {
  return useTranslation(ns);
}

/**
 * Change the active language at runtime.
 */
export function changeLanguage(lng: SupportedLanguage) {
  return i18n.changeLanguage(lng);
}

/**
 * Get the currently resolved language.
 */
export function getCurrentLanguage(): SupportedLanguage {
  // resolvedLanguage is more accurate after initialization
  const lng = (i18n.resolvedLanguage ||
    i18n.language ||
    "en") as SupportedLanguage;
  return SUPPORTED_LANGUAGES.includes(lng) ? lng : "en";
}

export default i18n;
