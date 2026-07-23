import { ExternalLink, ImageOff, Telescope } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import type { ParsedSpeciesPrediction } from "./fieldGuide";

export function SpeciesReferenceTrigger({ prediction }: { prediction: ParsedSpeciesPrediction }) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLSpanElement>(null);
  const closeTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [position, setPosition] = useState({ top: 0, left: 0 });
  const { t, i18n } = useI18n();
  const locale = useMemo(
    () =>
      (i18n.resolvedLanguage || i18n.language || "en").toLowerCase().startsWith("zh") ? "zh" : "en",
    [i18n.language, i18n.resolvedLanguage],
  );
  const referenceQuery = $api.useQuery(
    "get",
    "/api/v1/species/reference",
    {
      params: {
        query: {
          scientific_name: prediction.scientificName,
          common_name: prediction.commonName ?? prediction.displayName,
          locale,
        },
      },
    },
    {
      enabled: isOpen,
      staleTime: 24 * 60 * 60 * 1000,
      retry: 1,
    },
  );
  const reference = referenceQuery.data;

  const open = useCallback(() => {
    if (closeTimeoutRef.current) clearTimeout(closeTimeoutRef.current);
    const rect = triggerRef.current?.getBoundingClientRect();
    if (rect) setPosition({ top: rect.top, left: rect.right + 8 });
    setIsOpen(true);
  }, []);
  const close = useCallback(() => {
    closeTimeoutRef.current = setTimeout(() => setIsOpen(false), 150);
  }, []);

  useEffect(
    () => () => {
      if (closeTimeoutRef.current) clearTimeout(closeTimeoutRef.current);
    },
    [],
  );

  const tooltip = isOpen ? (
    <div
      style={{ position: "fixed", left: position.left, top: position.top, zIndex: "var(--z-tooltip)" as unknown as number }}
      className="w-[min(520px,calc(100vw-96px))] rounded-xl border border-white/12 bg-zinc-950/95 p-3 text-left text-white shadow-2xl shadow-black/40 backdrop-blur-xl"
      role="tooltip"
      onMouseEnter={open}
      onMouseLeave={close}
    >
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold text-white/86">
            {t("assets.photos.fullscreen.fieldGuide.reference")}
          </p>
          <p className="text-[11px] text-white/42">
            {t("assets.photos.fullscreen.fieldGuide.fromINaturalist")}
          </p>
        </div>
        {reference?.reference_url && (
          <a
            href={reference.reference_url}
            target="_blank"
            rel="noreferrer"
            className="grid size-7 shrink-0 place-items-center rounded-full bg-white/8 text-white/58 hover:bg-white/12 hover:text-white"
            aria-label={t("assets.photos.fullscreen.fieldGuide.openINaturalist")}
          >
            <ExternalLink className="size-3.5" />
          </a>
        )}
      </div>
      {referenceQuery.isLoading ? (
        <div className="grid grid-cols-[72px_1fr] gap-3">
          <div className="h-20 rounded-lg bg-white/10" />
          <div className="space-y-2.5">
            <div className="h-3.5 w-32 rounded-full bg-white/14" />
            <div className="h-3 w-full rounded-full bg-white/10" />
            <div className="h-3 w-5/6 rounded-full bg-white/10" />
          </div>
        </div>
      ) : reference ? (
        <div className="space-y-3">
          <div className="grid grid-cols-[72px_1fr] gap-3">
            {reference.image_url ? (
              <img
                src={reference.image_url}
                alt={reference.common_name ?? reference.scientific_name ?? prediction.displayName}
                className="h-20 w-18 rounded-lg object-cover"
                loading="lazy"
              />
            ) : (
              <div className="grid h-20 w-18 place-items-center rounded-lg bg-white/8 text-white/35">
                <ImageOff className="size-5" />
              </div>
            )}
            <div className="min-w-0">
              <h4 className="truncate text-sm font-semibold text-white/90">
                {reference.common_name ?? prediction.displayName}
              </h4>
              {reference.scientific_name && (
                <p className="truncate text-xs italic text-white/50">{reference.scientific_name}</p>
              )}
              {reference.wikipedia_summary && (
                <p className="mt-1.5 line-clamp-3 text-xs leading-5 text-white/62">
                  {reference.wikipedia_summary}
                </p>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-[11px] text-white/45">
            {reference.image_license && (
              <span className="rounded-full bg-white/8 px-2 py-1 uppercase text-white/55">
                {reference.image_license}
              </span>
            )}
            {reference.wikipedia_url && (
              <a
                href={reference.wikipedia_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 rounded-full bg-white/8 px-2 py-1 text-white/68 hover:bg-white/12 hover:text-white"
              >
                {t("assets.photos.fullscreen.fieldGuide.openWikipedia")}
                <ExternalLink className="size-3" />
              </a>
            )}
            {reference.image_attribution && (
              <span className="min-w-0 truncate">{reference.image_attribution}</span>
            )}
          </div>
        </div>
      ) : (
        <p className="text-xs leading-5 text-white/52">
          {t("assets.photos.fullscreen.fieldGuide.referenceError")}
        </p>
      )}
    </div>
  ) : null;

  return (
    <>
      <span
        ref={triggerRef}
        className="inline-flex shrink-0"
        onMouseEnter={open}
        onMouseLeave={close}
        onFocus={open}
        onBlur={() => setIsOpen(false)}
      >
        <button
          type="button"
          className="btn btn-soft btn-info btn-sm btn-circle"
          aria-label={t("assets.photos.fullscreen.fieldGuide.reference")}
        >
          <Telescope className="size-3.5" />
        </button>
      </span>
      {tooltip && createPortal(tooltip, document.body)}
    </>
  );
}
