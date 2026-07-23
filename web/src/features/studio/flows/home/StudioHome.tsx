import React from "react";
import {
  Clock,
  Crop,
  Frame,
  Image as ImageIcon,
  ImageOff,
  SlidersHorizontal,
  Type,
  type LucideIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import PageHeader from "@/components/ui/PageHeader";
import { RecentEditItem } from "./RecentEditItem";
import type { RecentEditRecord } from "../../state/recentEdits";
import type { EditorTab } from "../editor/EditorPanel";

type StudioHomeProps = {
  recent: RecentEditRecord[];
  onResume: (assetId: string) => void;
  /** Pick a photo, then open the editor on the given tool. */
  onOpenTool: (tab: EditorTab) => void;
  onClearRecent: () => void;
};

function ToolCard({
  icon: Icon,
  title,
  body,
  onClick,
}: {
  icon: LucideIcon;
  title: string;
  body: string;
  onClick: () => void;
}): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      className="group relative overflow-hidden rounded-xl border border-base-300 bg-base-100 p-5 text-left transition-all hover:border-primary/40 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-primary/50"
    >
      <div className="mb-3.5 grid h-11 w-11 place-items-center rounded-lg bg-primary/10 text-primary">
        <Icon size={22} />
      </div>
      <h3 className="text-base font-semibold text-base-content">{title}</h3>
      <p className="mt-1.5 text-sm text-base-content/55">{body}</p>
    </button>
  );
}

export function StudioHome({
  recent,
  onResume,
  onOpenTool,
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
        <button
          type="button"
          onClick={() => onOpenTool("develop")}
          className="btn btn-primary btn-sm gap-2"
        >
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
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <ToolCard
              icon={SlidersHorizontal}
              title={t("studio.home.adjust.title", { defaultValue: "Adjust" })}
              body={t("studio.home.adjust.body", {
                defaultValue: "Exposure, color, and detail with a lightweight color panel.",
              })}
              onClick={() => onOpenTool("develop")}
            />
            <ToolCard
              icon={Crop}
              title={t("studio.home.crop.title", { defaultValue: "Crop" })}
              body={t("studio.home.crop.body", {
                defaultValue: "Reframe with aspect presets, rotate, and flip.",
              })}
              onClick={() => onOpenTool("crop")}
            />
            <ToolCard
              icon={Frame}
              title={t("studio.home.frame.title", { defaultValue: "Add a Frame" })}
              body={t("studio.home.frame.body", {
                defaultValue: "Camera-branded presets, or your own border and captions.",
              })}
              onClick={() => onOpenTool("frame")}
            />
            <ToolCard
              icon={Type}
              title={t("studio.home.text.title", { defaultValue: "Text" })}
              body={t("studio.home.text.body", {
                defaultValue: "Add captions on the photo, with depth-aware placement.",
              })}
              onClick={() => onOpenTool("text")}
            />
          </div>
        </section>
      </div>
    </div>
  );
}
