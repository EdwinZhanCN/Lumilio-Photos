import { useSyncExternalStore } from "react";

const BREAKPOINTS = {
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
  "2xl": 1536,
} as const;

export type Breakpoint = keyof typeof BREAKPOINTS;

function subscribe(query: string, onChange: () => void): () => void {
  const mql = window.matchMedia(query);
  mql.addEventListener("change", onChange);
  return () => mql.removeEventListener("change", onChange);
}

/** True once the viewport is at or above the given Tailwind breakpoint. */
export function useBreakpoint(breakpoint: Breakpoint): boolean {
  const query = `(min-width: ${BREAKPOINTS[breakpoint]}px)`;
  return useSyncExternalStore(
    (onChange) => subscribe(query, onChange),
    () => window.matchMedia(query).matches,
    () => false,
  );
}

/** Convenience alias for the `lg` breakpoint — the desktop/mobile shell split. */
export function useIsMobile(): boolean {
  return !useBreakpoint("lg");
}
