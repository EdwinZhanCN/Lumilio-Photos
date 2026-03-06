import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { LumilioAvatar } from "@/features/lumilio/components/LumilioAvatar/LumilioAvatar";
import {
  LEGACY_THEME_STORAGE_KEY,
  THEME_STORAGE_KEY,
  THEME_STORAGE_VERSION,
} from "@/lib/settings/registry";

type ThemeMode = "dark" | "light";

interface ThemeEnvelope {
  version: number;
  data: ThemeMode;
}

function asThemeMode(value: unknown): ThemeMode | null {
  return value === "dark" || value === "light" ? value : null;
}

function parseStoredTheme(raw: string | null): {
  theme: ThemeMode | null;
  needsRewrite: boolean;
} {
  if (!raw) return { theme: null, needsRewrite: false };

  const literalTheme = asThemeMode(raw);
  if (literalTheme) {
    return { theme: literalTheme, needsRewrite: true };
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    const parsedTheme = asThemeMode(parsed);
    if (parsedTheme) {
      return { theme: parsedTheme, needsRewrite: true };
    }

    if (typeof parsed === "object" && parsed !== null) {
      const maybeEnvelope = parsed as Partial<ThemeEnvelope> & {
        theme?: unknown;
      };
      const envelopeTheme = asThemeMode(maybeEnvelope.data);
      if (envelopeTheme) {
        return {
          theme: envelopeTheme,
          needsRewrite: maybeEnvelope.version !== THEME_STORAGE_VERSION,
        };
      }

      const legacyObjectTheme = asThemeMode(maybeEnvelope.theme);
      if (legacyObjectTheme) {
        return { theme: legacyObjectTheme, needsRewrite: true };
      }
    }
  } catch {
    return { theme: null, needsRewrite: false };
  }

  return { theme: null, needsRewrite: false };
}

function persistTheme(theme: ThemeMode): void {
  if (typeof localStorage === "undefined") return;
  const payload: ThemeEnvelope = {
    version: THEME_STORAGE_VERSION,
    data: theme,
  };
  localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify(payload));
  localStorage.removeItem(LEGACY_THEME_STORAGE_KEY);
}

function resolveInitialTheme(): ThemeMode {
  if (typeof localStorage === "undefined") return "light";

  const primary = parseStoredTheme(localStorage.getItem(THEME_STORAGE_KEY));
  if (primary.theme) {
    if (primary.needsRewrite) {
      persistTheme(primary.theme);
    }
    return primary.theme;
  }

  const legacy = parseStoredTheme(
    localStorage.getItem(LEGACY_THEME_STORAGE_KEY),
  );
  if (legacy.theme) {
    persistTheme(legacy.theme);
    return legacy.theme;
  }

  return "light";
}

function NavBar() {
  const [isDarkMode, setIsDarkMode] = useState(false);
  const [isLumilioHovered, setIsLumilioHovered] = useState(false);
  const { t } = useI18n();

  // Initialize theme on component mount
  useEffect(() => {
    const initialTheme = resolveInitialTheme();
    document.documentElement.setAttribute("data-theme", initialTheme);
    setIsDarkMode(initialTheme === "dark");
  }, []);

  return (
    <div className="navbar bg-base-100 px-4 py-2 justify-between">
      {/* Branding */}
      <div className="flex-none">
        <Link className="btn btn-ghost text-xl" to="/">
          <img
            src={"/logo.png"}
            className="size-6 bg-contain object-contain "
            alt={t("app.name") + " Logo"}
          />
          {t("app.name")}
        </Link>
      </div>

      <div className="tooltip tooltip-bottom" data-tip={t("navbar.agent.open")}>
        <Link
          to="/lumilio"
          className="inline-flex items-center justify-center rounded-full p-1"
          aria-label={t("navbar.agent.label")}
          onMouseEnter={() => setIsLumilioHovered(true)}
          onMouseLeave={() => setIsLumilioHovered(false)}
        >
          <LumilioAvatar className="mb-2" size={0.2} start={isLumilioHovered} />
        </Link>
      </div>

      {/* Theme Controller */}
      <label className="swap swap-rotate">
        {/* this hidden checkbox controls the state */}
        <input
          type="checkbox"
          className="theme-controller"
          value="dark"
          checked={isDarkMode}
          onChange={(e) => {
            const newTheme = e.target.checked ? "dark" : "light";
            persistTheme(newTheme);
            document.documentElement.setAttribute("data-theme", newTheme);
            setIsDarkMode(e.target.checked);
          }}
        />

        {/* sun icon */}
        <svg
          className="swap-off h-6 w-6 fill-current"
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
        >
          <path d="M5.64,17l-.71.71a1,1,0,0,0,0,1.41,1,1,0,0,0,1.41,0l.71-.71A1,1,0,0,0,5.64,17ZM5,12a1,1,0,0,0-1-1H3a1,1,0,0,0,0,2H4A1,1,0,0,0,5,12Zm7-7a1,1,0,0,0,1-1V3a1,1,0,0,0-2,0V4A1,1,0,0,0,12,5ZM5.64,7.05a1,1,0,0,0,.7.29,1,1,0,0,0,0-1.41l-.71-.71A1,1,0,0,0,4.93,6.34Zm12,.29a1,1,0,0,0,.7-.29l.71-.71a1,1,0,1,0-1.41-1.41L17,5.64a1,1,0,0,0,0,1.41A1,1,0,0,0,17.66,7.34ZM21,11H20a1,1,0,0,0,0,2h1a1,1,0,0,0,0-2Zm-9,8a1,1,0,0,0-1,1v1a1,1,0,0,0,2,0V20A1,1,0,0,0,12,19ZM18.36,17A1,1,0,0,0,17,18.36l.71.71a1,1,0,0,0,1.41,0,1,1,0,0,0,0-1.41ZM12,6.5A5.5,5.5,0,1,0,17.5,12,5.51,5.51,0,0,0,12,6.5Zm0,9A3.5,3.5,0,1,1,15.5,12,3.5,3.5,0,0,1,12,15.5Z" />
        </svg>

        {/* moon icon */}
        <svg
          className="swap-on h-6 w-6 fill-current"
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
        >
          <path d="M21.64,13a1,1,0,0,0-1.05-.14,8.05,8.05,0,0,1-3.37.73A8.15,8.15,0,0,1,9.08,5.49a8.59,8.59,0,0,1,.25-2A1,1,0,0,0,8,2.36,10.14,10.14,0,1,0,22,14.05,1,1,0,0,0,21.64,13Zm-9.5,6.69A8.14,8.14,0,0,1,7.08,5.22v.27A10.15,10.15,0,0,0,17.22,15.63a9.79,9.79,0,0,0,2.1-.22A8.11,8.11,0,0,1,12.14,19.73Z" />
        </svg>
      </label>
    </div>
  );
}

export default NavBar;
