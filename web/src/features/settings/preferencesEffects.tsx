import { useEffect, type ReactNode } from "react";
import { changeLanguage, getCurrentLanguage } from "@/lib/i18n.tsx";
import { ThemeEffects } from "@/lib/theme/ThemeEffects";
import { usePreferencesStore } from "./preferences";

function LanguagePreferenceEffects({ children }: { children: ReactNode }) {
  const language = usePreferencesStore((s) => s.language);

  useEffect(() => {
    if (!language) return;

    if (document.documentElement.lang !== language) {
      document.documentElement.setAttribute("lang", language);
    }

    if (getCurrentLanguage() !== language) {
      void changeLanguage(language);
    }
  }, [language]);

  return children;
}

/** Mount at app root: theme document sync + language preference sync. */
export function PreferencesEffects({ children }: { children: ReactNode }) {
  return (
    <ThemeEffects>
      <LanguagePreferenceEffects>{children}</LanguagePreferenceEffects>
    </ThemeEffects>
  );
}
