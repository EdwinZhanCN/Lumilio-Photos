import { useCallback, useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Ellipsis } from "lucide-react";

type MenuPosition = { top: number; left: number };

export interface MenuItemProps {
  onSelect: () => void;
  disabled?: boolean;
  title?: string;
  /** Destructive entries are the only colored item in a menu. */
  danger?: boolean;
  /**
   * Why the entry is unavailable, shown as a tooltip on the entry itself. A
   * disabled item cannot be focused or hovered for its own `title`, so the
   * explanation rides on the list item, which still receives hover.
   */
  hint?: string;
  children: ReactNode;
}

/**
 * One menu entry. Carries the `role="none"` / `role="menuitem"` pair so callers
 * cannot produce a `role="menu"` whose children lack menu semantics.
 */
export function MenuItem({ onSelect, disabled, title, danger, hint, children }: MenuItemProps) {
  return (
    <li role="none" className={hint ? "tooltip tooltip-left" : undefined} data-tip={hint}>
      <button
        type="button"
        role="menuitem"
        disabled={disabled}
        title={title}
        className={danger ? "text-error" : undefined}
        onClick={onSelect}
      >
        {children}
      </button>
    </li>
  );
}

export interface AnchoredMenuProps {
  /** Accessible name of the trigger, e.g. "Actions for Family Archive". */
  label: string;
  /** Estimated menu box, used to flip above and clamp inside the viewport. */
  menuWidth?: number;
  menuHeight?: number;
  triggerClassName?: string;
  triggerIcon?: ReactNode;
  /**
   * Receives a `close` callback so a chosen item dismisses the menu. Build
   * entries with {@link MenuItem} (or `li.menu-title` for a section heading).
   */
  children: (helpers: { close: () => void }) => ReactNode;
}

/**
 * Overflow menu rendered through a portal with `position: fixed`, anchored to
 * its trigger's bounding rect.
 *
 * An in-tree dropdown is clipped here: a Storage Location section is
 * `overflow-hidden` for its rounded corners and the Repository table scrolls
 * horizontally, and both clip a positioned child. Rendering the menu in the
 * document body escapes every ancestor clip. It clamps to the viewport, flips
 * above when it would overflow the bottom, and closes on outside click, scroll,
 * resize, and Escape. Same mechanism as the Lumilio widget tile menu.
 */
export function AnchoredMenu({
  label,
  menuWidth = 240,
  menuHeight = 260,
  triggerClassName = "btn btn-square btn-ghost btn-xs",
  triggerIcon,
  children,
}: AnchoredMenuProps) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLUListElement>(null);
  const [position, setPosition] = useState<MenuPosition | null>(null);

  const close = useCallback(() => setPosition(null), []);

  const toggle = useCallback(() => {
    if (position) {
      close();
      return;
    }
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const left = Math.min(
      Math.max(8, rect.right - menuWidth),
      Math.max(8, window.innerWidth - menuWidth - 8),
    );
    let top = rect.bottom + 4;
    if (top + menuHeight > window.innerHeight - 8) {
      top = Math.max(8, rect.top - menuHeight - 4);
    }
    setPosition({ top, left });
  }, [position, close, menuWidth, menuHeight]);

  useEffect(() => {
    if (!position) return;
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (menuRef.current?.contains(target) || triggerRef.current?.contains(target)) return;
      close();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("mousedown", onPointerDown, true);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("mousedown", onPointerDown, true);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [position, close]);

  return (
    <>
      <button
        ref={triggerRef as RefObject<HTMLButtonElement>}
        type="button"
        className={triggerClassName}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={position != null}
        onClick={(event) => {
          event.stopPropagation();
          toggle();
        }}
      >
        {triggerIcon ?? <Ellipsis className="size-4" aria-hidden />}
      </button>
      {position
        ? createPortal(
            <ul
              ref={menuRef}
              role="menu"
              aria-label={label}
              style={{
                position: "fixed",
                top: position.top,
                left: position.left,
                width: menuWidth,
              }}
              className="menu menu-sm z-dropdown rounded-box border border-base-300 bg-base-100 p-2 shadow-xl"
            >
              {children({ close })}
            </ul>,
            document.body,
          )
        : null}
    </>
  );
}

export default AnchoredMenu;
