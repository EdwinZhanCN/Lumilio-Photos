import { useEffect, useState } from "react";
import type { DesktopPreferences } from "../../bindings/desktop/internal/control/dto/models.js";

export type ResolvedTheme = "light" | "dark";

function systemTheme(): ResolvedTheme {
  if (typeof window === "undefined" || !window.matchMedia) return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function resolveTheme(theme: string | undefined): ResolvedTheme {
  if (theme === "light" || theme === "dark") return theme;
  return systemTheme();
}

// Resolves the host theme preference (light/dark/system) against the OS
// preference. System mode tracks matchMedia live.
export function useTheme(preferences: DesktopPreferences | null): ResolvedTheme {
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolveTheme(preferences?.theme));

  useEffect(() => {
    const theme = preferences?.theme || "system";
    if (theme !== "system") {
      setResolved(resolveTheme(theme));
      return;
    }

    if (typeof window === "undefined" || !window.matchMedia) {
      setResolved("light");
      return;
    }

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      setResolved(media.matches ? "dark" : "light");
    };
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [preferences?.theme]);
  return resolved;
}
