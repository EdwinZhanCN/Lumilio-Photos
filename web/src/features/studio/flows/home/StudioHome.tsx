import React from "react";
import { Clock, Frame, Image as ImageIcon, ImageOff, SlidersHorizontal } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import PageHeader from "@/components/ui/PageHeader";
import { RecentEditItem } from "./RecentEditItem";
import type { RecentEditRecord } from "../../state/recentEdits";

type StudioHomeProps = {
  recent: RecentEditRecord[];
  onOpenEditor: () => void;
  onResume: (assetId: string) => void;
  onOpenBorderTool: () => void;
  onClearRecent: () => void;
};

export function StudioHome({
  recent,
  onOpenEditor,
  onResume,
  onOpenBorderTool,
  onClearRecent,
}: StudioHomeProps): React.JSX.Element {
  const { t } = useI18n();

  return (
    <div
      className="flex h-full min-h-0 w-full flex-col overflow-y-auto"
      data-screen-label="Studio Home"
    >
      <PageHeader
        title={t("studio.title", { defaultValue: "Studio" })}
        subtitle={t("studio.tagline", {
          defaultValue: "Quick, non-destructive edits for photos already in your library.",
        })}
        icon={<SlidersHorizontal className="h-6 w-6 text-primary" />}
      >
        <button type="button" onClick={onOpenEditor} className="btn btn-primary btn-sm gap-2">
          <ImageIcon size={16} />
          {t("studio.home.openEditor", { defaultValue: "Open editor" })}
        </button>
      </PageHeader>

      <div className="mx-auto w-full max-w-6xl px-4 py-6 lg:px-6">
        {/* Recent Edits */}
        <section className="mb-10">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-wider text-base-content/70">
              <Clock size={15} />
              {t("studio.home.recent", { defaultValue: "Recent Edits" })}
            </h2>
            {recent.length > 0 && (
              <button
                type="button"
                onClick={onClearRecent}
                className="btn btn-ghost btn-xs text-base-content/50"
              >
                {t("studio.home.clear", { defaultValue: "Clear" })}
              </button>
            )}
          </div>

          {recent.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-base-300 bg-base-100 px-6 py-14 text-center">
              <div className="mb-3 grid h-12 w-12 place-items-center rounded-full bg-base-200 text-base-content/40">
                <ImageOff size={22} />
              </div>
              <p className="text-sm font-medium text-base-content/80">
                {t("studio.home.empty.title", { defaultValue: "No recent edits yet" })}
              </p>
              <p className="mt-1 max-w-xs text-xs text-base-content/50">
                {t("studio.home.empty.body", {
                  defaultValue:
                    "Open a photo from your library to start editing. Your latest work shows up here.",
                })}
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 xl:grid-cols-3">
              {recent.map((item) => (
                <RecentEditItem key={item.assetId} item={item} onResume={onResume} />
              ))}
            </div>
          )}
        </section>

        {/* Tools */}
        <section>
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold uppercase tracking-wider text-base-content/70">
            <Frame size={15} />
            {t("studio.home.tools", { defaultValue: "Tools" })}
          </h2>
          <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
            <button
              type="button"
              onClick={onOpenBorderTool}
              className="group relative overflow-hidden rounded-xl border border-base-300 bg-base-100 p-6 text-left transition-all hover:border-primary/40 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-primary/50"
            >
              <div className="mb-4 grid h-11 w-11 place-items-center rounded-lg bg-primary/10 text-primary">
                <Frame size={22} />
              </div>
              <h3 className="text-base font-semibold text-base-content">
                {t("studio.home.border.title", { defaultValue: "Add a Border" })}
              </h3>
              <p className="mt-1.5 text-sm text-base-content/55">
                {t("studio.home.border.body", {
                  defaultValue:
                    "Frame a photo with colored, frosted, or vignette borders — applied on top of your edits.",
                })}
              </p>
            </button>
          </div>
        </section>
      </div>
    </div>
  );
}
