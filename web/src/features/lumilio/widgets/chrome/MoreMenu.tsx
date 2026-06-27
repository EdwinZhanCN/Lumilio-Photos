import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { MoreVertical, Pencil, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import type { WidgetSizeKey } from "../types";

interface MoreMenuProps {
  currentSize: WidgetSizeKey;
  onRename: () => void;
  onSize: (size: WidgetSizeKey) => void;
  onRemove: () => void;
  /** Over a photo (Cover) — use the glass trigger. */
  glass?: boolean;
}

const MENU_W = 208;
const MENU_H = 220;
const SIZES: WidgetSizeKey[] = ["s", "m", "l"];

/** Tile overflow menu. Rendered through a portal with `position: fixed` anchored
 * to the trigger's bounding rect — NOT an in-card dropdown. The board cell is
 * `overflow-hidden` (to clip cover/mosaic) AND react-grid-layout transforms each
 * cell, so an in-card absolute/fixed dropdown gets clipped or mis-positioned.
 * Clamps to the viewport, flips above when it would overflow the bottom, and
 * closes on outside-click / scroll / resize. */
export function MoreMenu({ currentSize, onRename, onSize, onRemove, glass }: MoreMenuProps) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLUListElement>(null);
  const [rect, setRect] = useState<{ top: number; left: number } | null>(null);

  const close = useCallback(() => setRect(null), []);

  const toggle = useCallback(() => {
    if (rect) {
      close();
      return;
    }
    const trigger = triggerRef.current;
    if (!trigger) return;
    const r = trigger.getBoundingClientRect();
    let left = Math.max(8, r.right - MENU_W);
    left = Math.min(left, window.innerWidth - MENU_W - 8);
    let top = r.bottom + 4;
    if (top + MENU_H > window.innerHeight - 8) top = Math.max(8, r.top - MENU_H - 4);
    setRect({ top, left });
  }, [rect, close]);

  useEffect(() => {
    if (!rect) return;
    const onPointer = (e: MouseEvent) => {
      const target = e.target as Node;
      if (menuRef.current?.contains(target) || triggerRef.current?.contains(target)) return;
      close();
    };
    window.addEventListener("mousedown", onPointer, true);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    return () => {
      window.removeEventListener("mousedown", onPointer, true);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
    };
  }, [rect, close]);

  const run = (fn: () => void) => () => {
    close();
    fn();
  };

  const triggerCls = glass
    ? "btn btn-xs btn-square border-0 bg-black/35 text-white hover:bg-black/55"
    : "btn btn-ghost btn-xs btn-square text-base-content/60";

  return (
    <div onMouseDown={(e) => e.stopPropagation()} onPointerDown={(e) => e.stopPropagation()}>
      <button
        ref={triggerRef}
        type="button"
        className={triggerCls}
        aria-label={t("lumilio.widgets.menu.more", "More")}
        aria-haspopup="menu"
        aria-expanded={!!rect}
        onClick={(e) => {
          e.stopPropagation();
          toggle();
        }}
      >
        <MoreVertical size={16} strokeWidth={1.85} />
      </button>
      {rect &&
        createPortal(
          <ul
            ref={menuRef}
            role="menu"
            style={{ position: "fixed", top: rect.top, left: rect.left, width: MENU_W }}
            className="menu menu-sm z-[60] gap-0.5 rounded-box border border-base-300 bg-base-100 p-1.5 text-base-content shadow-xl"
            onMouseDown={(e) => e.stopPropagation()}
          >
            <li>
              <button type="button" onClick={run(onRename)}>
                <Pencil size={16} strokeWidth={1.75} />
                {t("lumilio.widgets.menu.rename", "Rename")}
              </button>
            </li>
            <li className="menu-title px-2 pb-1 pt-2 text-base-content/45">
              {t("lumilio.widgets.menu.size", "Size")}
            </li>
            <li className="px-1">
              <div className="join w-full p-0">
                {SIZES.map((s) => (
                  <button
                    key={s}
                    type="button"
                    className={`btn btn-xs join-item flex-1 ${
                      currentSize === s
                        ? "btn-primary text-primary-content"
                        : "btn-ghost bg-base-200"
                    }`}
                    onClick={run(() => onSize(s))}
                  >
                    {s.toUpperCase()}
                  </button>
                ))}
              </div>
            </li>
            <div className="mx-1 my-1.5 h-px bg-base-200" />
            <li>
              <button
                type="button"
                className="text-error hover:bg-error/10"
                onClick={run(onRemove)}
              >
                <Trash2 size={16} strokeWidth={1.75} />
                {t("lumilio.widgets.menu.remove", "Remove")}
              </button>
            </li>
          </ul>,
          document.body,
        )}
    </div>
  );
}
