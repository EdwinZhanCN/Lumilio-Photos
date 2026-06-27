import type { LucideIcon } from "lucide-react";

/** Theme tone for an icon card, mapped to pure daisyUI tokens. */
export type RailCardTone = "primary" | "warning" | "accent";

const TONES: Record<RailCardTone, { gradient: string; text: string }> = {
  primary: {
    gradient: "from-primary/20 via-primary/10 to-base-200",
    text: "text-primary",
  },
  warning: {
    gradient: "from-warning/20 via-warning/10 to-base-200",
    text: "text-warning",
  },
  accent: {
    gradient: "from-accent/20 via-secondary/10 to-base-200",
    text: "text-accent",
  },
};

/**
 * The two card shapes used across every Collections rail:
 * - `icon`: a tinted gradient tile with a centered glyph and label —
 *   navigation/action entries (utilities, "open map"). No caption.
 * - `photo`: a square cover image with the caption overlaid at the bottom over a
 *   dark gradient; falls back to a centered glyph when there is no image. The
 *   caption truncates and exposes the full text via a tooltip.
 */
export type RailCardMedia =
  | { kind: "icon"; icon: LucideIcon; tone?: RailCardTone }
  | { kind: "photo"; src?: string | null; fallbackIcon?: LucideIcon };

export type RailCardProps = {
  media: RailCardMedia;
  title: string;
  /** Overlay caption second line — photo cards only; icon cards have no caption. */
  subtitle?: string;
  onClick?: () => void;
  /** Sizing (e.g. `w-48 shrink-0` in a rail, `w-full` in a grid). */
  className?: string;
};

function IconBody({
  icon: Icon,
  tone = "primary",
  title,
}: {
  icon: LucideIcon;
  tone?: RailCardTone;
  title: string;
}) {
  const toneStyle = TONES[tone];
  return (
    <div
      className={`relative aspect-square overflow-hidden rounded-[1.75rem] bg-gradient-to-br ${toneStyle.gradient} transition duration-300`}
    >
      <div className="flex h-full flex-col items-center justify-center gap-2 px-3">
        <Icon className={`size-10 ${toneStyle.text}`} strokeWidth={1.5} />
        <span
          className={`max-w-full truncate text-sm font-semibold ${toneStyle.text}`}
          title={title}
        >
          {title}
        </span>
      </div>
    </div>
  );
}

function PhotoBody({
  src,
  fallbackIcon: Fallback,
  title,
  subtitle,
}: {
  src?: string | null;
  fallbackIcon?: LucideIcon;
  title: string;
  subtitle?: string;
}) {
  return (
    <div className="relative aspect-square overflow-hidden rounded-[1.75rem] bg-base-200 transition duration-300">
      {src ? (
        <img
          src={src}
          alt={title}
          className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]"
          loading="lazy"
        />
      ) : (
        <div className="flex h-full items-center justify-center bg-gradient-to-br from-base-200 via-base-300/70 to-base-200">
          {Fallback && <Fallback className="size-12 text-base-content/35" />}
        </div>
      )}
      {/* The dark gradient backs the caption so white text stays legible over
          any cover or the light fallback. */}
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/40 to-transparent p-3 pt-10">
        <p
          className="truncate text-sm font-bold text-white drop-shadow-sm"
          title={title}
        >
          {title}
        </p>
        {subtitle && (
          <p className="mt-0.5 truncate text-xs text-white/70" title={subtitle}>
            {subtitle}
          </p>
        )}
      </div>
    </div>
  );
}

/**
 * Shared card for the Collections rails. Layout/width is owned by the caller via
 * `className`; the visual shape is chosen by `media.kind`.
 */
export default function RailCard({
  media,
  title,
  subtitle,
  onClick,
  className = "",
}: RailCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`group shrink-0 cursor-pointer text-left ${className}`}
    >
      {media.kind === "icon" ? (
        <IconBody icon={media.icon} tone={media.tone} title={title} />
      ) : (
        <PhotoBody
          src={media.src}
          fallbackIcon={media.fallbackIcon}
          title={title}
          subtitle={subtitle}
        />
      )}
    </button>
  );
}
