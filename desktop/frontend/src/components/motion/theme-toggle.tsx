import { Moon, Sun } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useEffect, type ButtonHTMLAttributes } from "react";
import { EASE_OUT_CSS } from "@/lib/ease";
import { cn } from "@/lib/utils";

export type ThemeVariant = "rectangle" | "circle" | "circle-blur" | "blinds";
export type ThemeToggleStart =
  | "top-left"
  | "top-right"
  | "bottom-left"
  | "bottom-right"
  | "center"
  | "bottom-up";

export interface ThemeToggleProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children" | "onClick"> {
  isDark: boolean;
  onToggle: () => void;
  variant?: ThemeVariant;
  start?: ThemeToggleStart;
  iconClassName?: string;
}

const VIEW_TRANSITION_STYLE_ID = "lumilio-theme-toggle-vt";

const VIEW_TRANSITION_CSS = `
html[data-lumilio-vt="rect"]::view-transition-old(root) {
  animation: none;
  mix-blend-mode: normal;
}
html[data-lumilio-vt="rect"]::view-transition-new(root) {
  animation: lumilio-rect-reveal 400ms ease-out;
  mix-blend-mode: normal;
}
html[data-lumilio-vt="circle"]::view-transition-old(root),
html[data-lumilio-vt="circle-blur"]::view-transition-old(root) {
  animation: none;
  mix-blend-mode: normal;
}
html[data-lumilio-vt="circle"]::view-transition-new(root) {
  animation: lumilio-circle-reveal 700ms cubic-bezier(0.4, 0, 0.2, 1);
  mix-blend-mode: normal;
}
html[data-lumilio-vt="circle-blur"]::view-transition-new(root) {
  animation: lumilio-circle-blur-reveal 700ms cubic-bezier(0.4, 0, 0.2, 1);
  mix-blend-mode: normal;
}
html[data-lumilio-vt="blinds"]::view-transition-old(root) {
  animation: none;
  mix-blend-mode: normal;
}
html[data-lumilio-vt="blinds"]::view-transition-new(root) {
  animation: lumilio-blinds-reveal 700ms ${EASE_OUT_CSS};
  mask-image: linear-gradient(90deg, #000 0 var(--lumilio-vt-slat), transparent calc(var(--lumilio-vt-slat) + 20px));
  mask-repeat: repeat;
  mask-size: 72px 100%;
  mix-blend-mode: normal;
}
@property --lumilio-vt-slat {
  syntax: "<length>";
  inherits: false;
  initial-value: 72px;
}
@keyframes lumilio-rect-reveal {
  from { clip-path: var(--lumilio-vt-from, inset(100% 0 0 0)); }
  to { clip-path: inset(0 0 0 0); }
}
@keyframes lumilio-circle-reveal {
  from { clip-path: circle(0% at var(--lumilio-vt-origin, 50% 100%)); }
  to { clip-path: circle(150% at var(--lumilio-vt-origin, 50% 100%)); }
}
@keyframes lumilio-circle-blur-reveal {
  from { clip-path: circle(0% at var(--lumilio-vt-origin, 50% 100%)); filter: blur(8px); }
  to { clip-path: circle(150% at var(--lumilio-vt-origin, 50% 100%)); filter: blur(0); }
}
@keyframes lumilio-blinds-reveal {
  from { --lumilio-vt-slat: -20px; }
  to { --lumilio-vt-slat: 72px; }
}
`;

const RECT_FROM: Record<ThemeToggleStart, string> = {
  "top-left": "inset(0 100% 100% 0)",
  "top-right": "inset(0 0 100% 100%)",
  "bottom-left": "inset(100% 100% 0 0)",
  "bottom-right": "inset(100% 0 0 100%)",
  center: "inset(50% 50% 50% 50%)",
  "bottom-up": "inset(100% 0 0 0)",
};

const CIRCLE_ORIGIN: Record<ThemeToggleStart, string> = {
  "top-left": "0% 0%",
  "top-right": "100% 0%",
  "bottom-left": "0% 100%",
  "bottom-right": "100% 100%",
  center: "50% 50%",
  "bottom-up": "50% 100%",
};

type ViewTransitionDocument = Document & {
  startViewTransition?: (update: () => void) => { finished: Promise<void> };
};

function useViewTransitionToggle(
  onToggle: () => void,
  variant: ThemeVariant,
  start: ThemeToggleStart,
) {
  const reduce = useReducedMotion() ?? false;

  useEffect(() => {
    if (document.getElementById(VIEW_TRANSITION_STYLE_ID)) return;
    const style = document.createElement("style");
    style.id = VIEW_TRANSITION_STYLE_ID;
    style.textContent = VIEW_TRANSITION_CSS;
    document.head.appendChild(style);
  }, []);

  return () => {
    const viewTransitionDocument = document as ViewTransitionDocument;
    if (reduce || !viewTransitionDocument.startViewTransition) {
      onToggle();
      return;
    }

    const root = document.documentElement;
    if (variant === "rectangle") {
      root.style.setProperty("--lumilio-vt-from", RECT_FROM[start]);
      root.dataset.lumilioVt = "rect";
    } else if (variant === "blinds") {
      root.dataset.lumilioVt = "blinds";
    } else {
      root.style.setProperty("--lumilio-vt-origin", CIRCLE_ORIGIN[start]);
      root.dataset.lumilioVt = variant;
    }

    const transition = viewTransitionDocument.startViewTransition(() => onToggle());
    void transition.finished.finally(() => {
      delete root.dataset.lumilioVt;
      root.style.removeProperty("--lumilio-vt-from");
      root.style.removeProperty("--lumilio-vt-origin");
    });
  };
}

/** beUI's theme-toggle interaction, controlled by Desktop host preferences. */
export function ThemeToggle({
  isDark,
  onToggle,
  variant = "circle-blur",
  start = "bottom-up",
  className,
  iconClassName = "size-4",
  "aria-label": ariaLabel,
  ...rest
}: ThemeToggleProps) {
  const toggle = useViewTransitionToggle(onToggle, variant, start);

  return (
    <button
      type="button"
      aria-label={ariaLabel ?? (isDark ? "Switch to light mode" : "Switch to dark mode")}
      onClick={toggle}
      className={cn(
        "flex items-center justify-center",
        className,
      )}
      {...rest}
    >
      <span className="relative grid place-items-center overflow-hidden">
        <AnimatePresence mode="popLayout" initial={false}>
          <motion.span
            key={isDark ? "dark" : "light"}
            initial={{ opacity: 0, scale: 0.25, filter: "blur(8px)" }}
            animate={{ opacity: 1, scale: 1, filter: "blur(0px)" }}
            exit={{ opacity: 0, scale: 0.25, filter: "blur(8px)" }}
            transition={{ duration: 0.2, ease: "easeInOut" }}
            className="col-start-1 row-start-1 inline-flex"
          >
            {isDark ? <Sun className={iconClassName} /> : <Moon className={iconClassName} />}
          </motion.span>
        </AnimatePresence>
      </span>
    </button>
  );
}
