import type { ReactNode } from "react";

interface TileBodyProps {
  children: ReactNode;
  /** When set, the body is a clickable deep-link region. */
  clickable?: boolean;
  onActivate?: () => void;
}

/** The tile content wrapper. MUST stay `relative flex-1 min-h-0 overflow-hidden
 * flex flex-col` — without `flex flex-col` the `flex-1` children of Stat / Error
 * / Empty collapse to the top instead of filling and centering vertically
 * (regression-prone; see handoff). Header controls inside stopPropagation so
 * they don't trip the body's deep-link. We navigate programmatically rather
 * than wrapping in an <a> so floating controls (hover switcher) can't trigger a
 * native anchor navigation. */
export function TileBody({ children, clickable, onActivate }: TileBodyProps) {
  return (
    <div
      className={`relative flex min-h-0 flex-1 flex-col overflow-hidden ${
        clickable ? "cursor-pointer" : ""
      }`}
      role={clickable ? "button" : undefined}
      tabIndex={clickable ? 0 : undefined}
      onClick={clickable ? onActivate : undefined}
      onKeyDown={
        clickable
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onActivate?.();
              }
            }
          : undefined
      }
    >
      {children}
    </div>
  );
}
