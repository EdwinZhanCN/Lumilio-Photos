import { useI18n } from "@/lib/i18n.tsx";
import { listWidgets } from "../registry";

/** Localized name for a view. Static t() calls (not t(def.label)) so the i18n
 * extractor can see them; unknown views fall back to their type id. */
function useViewName() {
  const { t } = useI18n();
  return (type: string): string => {
    switch (type) {
      case "cover_card":
        return t("lumilio.widgets.view.cover", "Cover");
      case "number_card":
        return t("lumilio.widgets.view.number", "Stat");
      case "spark_card":
        return t("lumilio.widgets.view.spark", "Timeline");
      case "mosaic_card":
        return t("lumilio.widgets.view.mosaic", "Mosaic");
      default:
        return type;
    }
  };
}

interface ViewSwitcherProps {
  /** Currently rendered view (widget type). */
  current: string;
  /** Switch to another view. No-op is fine when current is re-selected. */
  onChange: (view: string) => void;
  /** "glass" floats over a photo (Cover); "default" sits on a light surface. */
  variant?: "default" | "glass";
  /** Larger buttons on the L tier. */
  size?: "sm" | "xs";
}

/** Segmented icon control that switches which View a Widget renders through. A
 * Widget is a window pinned to a ref; every registered view consumes the same
 * data, so switching is purely a presentation change. Reused by board cells
 * (persisted) and inline chat widgets (local until pinned). */
export function ViewSwitcher({ current, onChange, variant = "default", size = "sm" }: ViewSwitcherProps) {
  const { t } = useI18n();
  const viewName = useViewName();
  const views = listWidgets();
  if (views.length < 2) return null;

  const glass = variant === "glass";
  const btnSize = size === "sm" ? "btn-sm" : "btn-xs";
  const iconSize = size === "sm" ? 16 : 14;

  return (
    <div
      className={`join ${glass ? "shadow-sm" : ""}`}
      role="group"
      aria-label={t("lumilio.widgets.view.switcher", "Switch view")}
      onMouseDown={(e) => e.stopPropagation()}
      onPointerDown={(e) => e.stopPropagation()}
    >
      {views.map(({ type, icon: Icon }) => {
        const active = type === current;
        const name = viewName(type);
        const tone = glass
          ? active
            ? "btn-primary text-primary-content border-0"
            : "border-0 bg-black/35 text-white/90 hover:bg-black/55"
          : active
            ? "btn-primary text-primary-content"
            : "btn-ghost bg-base-200/70 text-base-content/70 hover:bg-base-300";
        return (
          <button
            key={type}
            type="button"
            className={`btn join-item px-2 ${btnSize} ${tone}`}
            aria-pressed={active}
            title={name}
            aria-label={name}
            onClick={(e) => {
              e.stopPropagation();
              if (!active) onChange(type);
            }}
          >
            <Icon size={iconSize} strokeWidth={1.85} />
          </button>
        );
      })}
    </div>
  );
}
