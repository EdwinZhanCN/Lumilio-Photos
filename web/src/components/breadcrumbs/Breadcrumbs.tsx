import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { useBreadcrumbItems } from "./BreadcrumbContext";

/**
 * Shared daisyUI breadcrumb renderer. Reads the trail published by the current
 * page via {@link useBreadcrumbs} and renders nothing when the trail is empty,
 * so it can live in a fixed app-shell slot without leaving a blank bar on pages
 * that don't opt in. Uses pure lumilio/daisyUI theme tokens.
 */
export function Breadcrumbs({ className = "" }: { className?: string }): ReactNode {
  const items = useBreadcrumbItems();
  if (items.length === 0) return null;

  return (
    <nav className={`breadcrumbs px-4 py-1.5 text-sm ${className}`} aria-label="Breadcrumb">
      <ul>
        {items.map((item, i) => {
          const isLast = i === items.length - 1;
          return (
            <li key={`${item.to ?? "current"}-${i}`}>
              {item.to && !isLast ? (
                <Link
                  to={item.to}
                  state={item.state}
                  className="link link-hover text-base-content/70 hover:text-base-content"
                >
                  {item.label}
                </Link>
              ) : (
                <span
                  className={isLast ? "font-medium text-base-content" : "text-base-content/70"}
                  aria-current={isLast ? "page" : undefined}
                >
                  {item.label}
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

export default Breadcrumbs;
