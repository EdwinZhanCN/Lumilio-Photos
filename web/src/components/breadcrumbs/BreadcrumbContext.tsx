import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useLocation } from "react-router-dom";

/**
 * One node in the navigation trail. A node with `to` renders as a link to a
 * parent; the node without `to` (conventionally the last) is the current page.
 * `state` is forwarded to the router so cross-context returns (e.g. a widget
 * that jumped into the library) can carry their origin.
 */
export interface BreadcrumbItem {
  /** Visible label. Already localized — the page owns translation. */
  label: string;
  /** Target path; omit for the current (non-link) page. */
  to?: string;
  /** Optional router state forwarded to the `<Link>`. */
  state?: unknown;
}

interface BreadcrumbContextValue {
  items: BreadcrumbItem[];
  /** Publish the crumbs for the current page. Pages call this via
   * {@link useBreadcrumbs}; calling with `[]` clears the trail. */
  setItems: (items: BreadcrumbItem[]) => void;
}

// Defaults to a no-op so pages can publish crumbs in isolation (e.g. unit
// tests) without mounting the whole app shell; only the renderer cares whether
// a real provider is present, and it simply shows nothing when it isn't.
const BreadcrumbContext = createContext<BreadcrumbContextValue>({
  items: [],
  setItems: () => {},
});

/**
 * Holds the crumbs declared by whatever page is mounted. The trail is *not* a
 * manually maintained history stack: it is derived state owned by the page, so
 * it stays refresh- and deep-link-safe. We clear it during render whenever the
 * pathname changes so a page that declares no crumbs never inherits the
 * previous page's trail.
 */
export function BreadcrumbProvider({ children }: { children: ReactNode }): ReactNode {
  const { pathname } = useLocation();
  const [items, setItems] = useState<BreadcrumbItem[]>([]);
  const prevPath = useRef(pathname);

  // Render-phase reset: runs before any child effect publishes its crumbs, so
  // there is no flash of stale-then-clear and no effect-ordering race.
  if (prevPath.current !== pathname) {
    prevPath.current = pathname;
    if (items.length > 0) setItems([]);
  }

  const value = useMemo<BreadcrumbContextValue>(() => ({ items, setItems }), [items]);

  return <BreadcrumbContext.Provider value={value}>{children}</BreadcrumbContext.Provider>;
}

/** Read the current trail. Used by the shared `<Breadcrumbs />` renderer. */
export function useBreadcrumbItems(): BreadcrumbItem[] {
  return useContext(BreadcrumbContext).items;
}

/**
 * Publish the breadcrumb trail for the current page. The trail is cleared on
 * unmount, so pages that don't call this hook show no breadcrumbs.
 *
 * ```tsx
 * useBreadcrumbs([
 *   { label: t("nav.home", "Home"), to: "/" },
 *   { label: t("collections.title", "Collections"), to: "/collections" },
 *   { label: album?.title ?? t("collections.album", "Album") },
 * ]);
 * ```
 */
export function useBreadcrumbs(items: BreadcrumbItem[]): void {
  const { setItems } = useContext(BreadcrumbContext);
  // Structural dependency: lets callers pass an inline array literal without
  // re-running the effect on every render.
  const key = JSON.stringify(items);
  const setRef = useRef(setItems);
  setRef.current = setItems;
  const itemsRef = useRef(items);
  itemsRef.current = items;

  useEffect(() => {
    setRef.current(itemsRef.current);
    return () => setRef.current([]);
  }, [key]);
}

/** Imperative escape hatch for non-render flows; prefer {@link useBreadcrumbs}. */
export function useSetBreadcrumbs(): (items: BreadcrumbItem[]) => void {
  const { setItems } = useContext(BreadcrumbContext);
  return useCallback((items: BreadcrumbItem[]) => setItems(items), [setItems]);
}
