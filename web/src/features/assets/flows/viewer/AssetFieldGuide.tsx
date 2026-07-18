import { Bird, ChevronUp, ScanSearch, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import {
  formatSpeciesScore,
  getSpeciesScorePercent,
  TAXONOMY_RANKS,
  type ParsedSpeciesPrediction,
  type TaxonomyRank,
} from "./fieldGuide";
import { SpeciesReferenceTrigger } from "./SpeciesReferenceTrigger";

function getRankLabel(t: (key: string) => string, rank: TaxonomyRank) {
  return t(`assets.photos.fullscreen.fieldGuide.ranks.${rank}`);
}

type AssetFieldGuideProps = {
  open: boolean;
  loading: boolean;
  error: boolean;
  predictions: ParsedSpeciesPrediction[];
  onToggle: () => void;
};

export function AssetFieldGuide({
  open,
  loading,
  error,
  predictions,
  onToggle,
}: AssetFieldGuideProps) {
  const { t } = useI18n();
  const primary = predictions[0];
  const available = predictions.length > 0;

  return (
    <>
      {open && (
        <aside className="absolute left-6 bottom-28 z-20 max-h-[calc(100vh-9rem)] w-[calc(100vw-48px)] max-w-[420px] overflow-y-auto rounded-2xl border border-white/10 bg-zinc-950/78 text-white shadow-2xl shadow-emerald-950/30 backdrop-blur-2xl">
          <div className="p-5">
            <div className="mb-5 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="grid size-9 place-items-center rounded-full bg-emerald-400/15 text-emerald-200 ring-1 ring-emerald-300/20">
                  <ScanSearch className="size-5" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold tracking-wide">
                    {t("assets.photos.fullscreen.fieldGuide.topLabels")}
                  </h2>
                  <p className="text-xs text-white/45">
                    {loading
                      ? t("assets.photos.fullscreen.fieldGuide.loading")
                      : error
                        ? t("assets.photos.fullscreen.fieldGuide.loadError")
                        : available
                          ? t("assets.photos.fullscreen.fieldGuide.predictionsCount", {
                              count: predictions.length,
                            })
                          : t("assets.photos.fullscreen.fieldGuide.noResults")}
                  </p>
                </div>
              </div>
              <button
                type="button"
                className="btn btn-circle btn-ghost btn-xs text-white/55 hover:bg-white/10 hover:text-white"
                onClick={onToggle}
                aria-label={t("common.close")}
              >
                <X className="size-4" />
              </button>
            </div>

            <div className="space-y-4">
              {loading ? (
                [0, 1, 2].map((item) => (
                  <div key={item} className="grid grid-cols-[28px_1fr] gap-3">
                    <div className="grid size-6 place-items-center rounded-full bg-white/10 text-xs font-semibold text-white/70">
                      {item + 1}
                    </div>
                    <div className="min-w-0">
                      <div className="mb-2 flex items-center justify-between gap-4">
                        <div className="h-4 w-40 rounded-full bg-white/14" />
                        <div className="h-4 w-10 rounded-full bg-white/18" />
                      </div>
                      <div className="mb-3 h-3 w-28 rounded-full bg-white/10" />
                      <div className="h-1.5 rounded-full bg-white/12" />
                    </div>
                  </div>
                ))
              ) : available ? (
                predictions.map((prediction, index) => (
                  <div
                    key={`${prediction.label}-${index}`}
                    className="grid grid-cols-[28px_1fr] gap-3"
                  >
                    <div className="grid size-6 place-items-center rounded-full bg-white/10 text-xs font-semibold text-white/70">
                      {index + 1}
                    </div>
                    <div className="min-w-0">
                      <div className="mb-1.5 flex items-start justify-between gap-4">
                        <div className="min-w-0">
                          <div className="flex min-w-0 items-center gap-1.5">
                            <h3 className="truncate text-sm font-semibold leading-5">
                              {prediction.displayName}
                            </h3>
                            <SpeciesReferenceTrigger prediction={prediction} />
                          </div>
                          {prediction.scientificName !== prediction.displayName && (
                            <p className="truncate text-xs italic text-white/50">
                              {prediction.scientificName}
                            </p>
                          )}
                        </div>
                        <span className="shrink-0 text-sm font-semibold text-white/88">
                          {formatSpeciesScore(prediction.score)}
                        </span>
                      </div>
                      <div className="h-1.5 rounded-full bg-white/12">
                        <div
                          className="h-full rounded-full bg-gradient-to-r from-emerald-300 via-lime-300 to-white/85"
                          style={{ width: `${getSpeciesScorePercent(prediction.score)}%` }}
                        />
                      </div>
                    </div>
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-white/10 bg-white/5 px-4 py-5 text-sm text-white/58">
                  {t("assets.photos.fullscreen.fieldGuide.noResults")}
                </div>
              )}
            </div>
          </div>

          <div className="border-t border-white/10 p-5">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Bird className="size-5 text-emerald-200" />
                <h3 className="text-sm font-semibold">
                  {t("assets.photos.fullscreen.fieldGuide.taxonomy")}
                </h3>
              </div>
              <ChevronUp className="size-4 text-white/55" />
            </div>
            <div className="space-y-3">
              {TAXONOMY_RANKS.map((rank) => {
                const value = primary?.taxonomy[rank];
                return (
                  <div
                    key={rank}
                    className="grid grid-cols-[104px_1fr] items-center border-b border-white/10 pb-2 text-sm last:border-b-0 last:pb-0"
                  >
                    <span className="text-white/48">{getRankLabel(t, rank)}</span>
                    <span className={value ? "truncate text-white/88" : "text-white/28"}>
                      {value ?? "-"}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </aside>
      )}

      {available && (
        <button
          type="button"
          className={`group absolute bottom-6 left-6 z-30 flex h-16 min-w-16 items-center gap-3 rounded-full border px-5 text-white shadow-2xl backdrop-blur-xl transition duration-200 ${
            open
              ? "border-emerald-200/55 bg-emerald-400/22 shadow-emerald-500/25"
              : "border-white/15 bg-zinc-950/64 shadow-black/40 hover:border-emerald-200/45 hover:bg-emerald-400/18 hover:shadow-emerald-500/20"
          }`}
          onClick={onToggle}
          aria-label={t("assets.photos.fullscreen.fieldGuide.open")}
          title={t("assets.photos.fullscreen.fieldGuide.open")}
        >
          <span className="grid size-9 place-items-center rounded-full bg-emerald-300 text-zinc-950 ring-4 ring-emerald-300/18 transition group-hover:scale-105">
            <Bird className="size-5" />
          </span>
          <span className="hidden pr-1 text-sm font-semibold tracking-wide sm:inline">
            {t("assets.photos.fullscreen.fieldGuide.button")}
          </span>
        </button>
      )}
    </>
  );
}
