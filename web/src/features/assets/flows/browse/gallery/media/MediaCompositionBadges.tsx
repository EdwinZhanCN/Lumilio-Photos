import { memo } from "react";
import { useI18n } from "@/lib/i18n";
import type { MediaCompositionFacts } from "../../../../types";

/**
 * Composition badges describe what a logical media item is made of. They are
 * deliberately textual: a JPEG+RAW pair is one media item, never a stack, so it
 * must not borrow the stack icon.
 */
function compositionLabels(composition: MediaCompositionFacts): string[] {
  const labels: string[] = [];
  if (composition.hasRaw) labels.push(composition.hasJpeg ? "JPEG+RAW" : "RAW");
  if (composition.hasLiveMotion) labels.push("LIVE");
  if (composition.hasEdited) labels.push("EDITED");
  return labels;
}

interface MediaCompositionBadgesProps {
  composition: MediaCompositionFacts | undefined;
  /** Video thumbnails already occupy the top-left corner. */
  offsetTop?: boolean;
}

export const MediaCompositionBadges = memo(function MediaCompositionBadges({
  composition,
  offsetTop = false,
}: MediaCompositionBadgesProps) {
  const { t } = useI18n();
  const labels = composition ? compositionLabels(composition) : [];

  if (labels.length === 0) return null;

  return (
    <div
      className={`pointer-events-none absolute left-3 flex flex-wrap gap-1 ${
        offsetTop ? "top-10" : "top-3"
      }`}
      aria-label={t("assets.mediaThumbnail.composition_sr_only", "Media composition")}
    >
      {labels.map((label) => (
        <span
          key={label}
          className="rounded-full border border-white/15 bg-black/55 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-white backdrop-blur-sm"
        >
          {label}
        </span>
      ))}
    </div>
  );
});

export default MediaCompositionBadges;
