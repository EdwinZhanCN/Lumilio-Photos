import React from "react";
import {
  ArrowLeft,
  Download,
  Eye,
  EyeOff,
  Info,
  Loader2,
  PanelLeft,
  PanelRight,
  RotateCw,
  Save,
  SlidersHorizontal,
  Undo2,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";

type TopBarProps = {
  fileName: string;
  canUndo: boolean;
  beforeActive: boolean;
  isSaving: boolean;
  isExporting: boolean;
  /** Desktop panel visibility (drives the toggle icon state). */
  leftOpen: boolean;
  rightOpen: boolean;
  onBack: () => void;
  onUndo: () => void;
  onResetAll: () => void;
  onBeforeDown: () => void;
  onBeforeUp: () => void;
  onSave: () => void;
  onExport: () => void;
  /** Desktop: collapse/expand the side rails. */
  onToggleLeft: () => void;
  onToggleRight: () => void;
  /** Mobile: open the info / editor bottom sheets. */
  onOpenInfo: () => void;
  onOpenEdit: () => void;
};

export function TopBar({
  fileName,
  canUndo,
  beforeActive,
  isSaving,
  isExporting,
  leftOpen,
  rightOpen,
  onBack,
  onUndo,
  onResetAll,
  onBeforeDown,
  onBeforeUp,
  onSave,
  onExport,
  onToggleLeft,
  onToggleRight,
  onOpenInfo,
  onOpenEdit,
}: TopBarProps): React.JSX.Element {
  const { t } = useI18n();

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b border-base-300 bg-base-100 px-2 sm:gap-3 sm:px-3">
      <button
        type="button"
        onClick={onBack}
        className="btn btn-ghost btn-sm gap-2 text-base-content/80"
      >
        <ArrowLeft size={16} />
        <span className="hidden sm:inline">
          {t("studio.editor.change", { defaultValue: "Change Photo" })}
        </span>
      </button>

      {/* Desktop: collapse the info rail. Mobile: open the info sheet. */}
      <div
        className="tooltip tooltip-bottom hidden lg:block"
        data-tip={t("studio.panel.info", { defaultValue: "Info panel" })}
      >
        <button
          type="button"
          onClick={onToggleLeft}
          aria-pressed={leftOpen}
          aria-label={t("studio.panel.info", { defaultValue: "Info panel" })}
          className={`btn btn-ghost btn-sm btn-square ${
            leftOpen ? "text-base-content/80" : "text-base-content/40"
          }`}
        >
          <PanelLeft size={17} />
        </button>
      </div>
      <button
        type="button"
        onClick={onOpenInfo}
        aria-label={t("studio.panel.info", { defaultValue: "Info panel" })}
        className="btn btn-ghost btn-sm btn-square text-base-content/70 lg:hidden"
      >
        <Info size={17} />
      </button>

      <div className="hidden h-6 w-px bg-base-300 sm:block" />

      <div className="flex min-w-0 items-center">
        <span className="truncate text-sm font-medium text-base-content">{fileName}</span>
      </div>

      <div className="ml-auto flex items-center gap-1 sm:gap-1.5">
        <div
          className="tooltip tooltip-bottom"
          data-tip={t("studio.editor.undo", { defaultValue: "Undo" })}
        >
          <button
            type="button"
            onClick={onUndo}
            disabled={!canUndo}
            aria-label={t("studio.editor.undo", { defaultValue: "Undo" })}
            className="btn btn-ghost btn-sm btn-square text-base-content/70"
          >
            <Undo2 size={17} />
          </button>
        </div>
        <div
          className="tooltip tooltip-bottom"
          data-tip={t("studio.editor.resetAll", { defaultValue: "Reset all" })}
        >
          <button
            type="button"
            onClick={onResetAll}
            aria-label={t("studio.editor.resetAll", { defaultValue: "Reset all" })}
            className="btn btn-ghost btn-sm btn-square text-base-content/70"
          >
            <RotateCw size={16} />
          </button>
        </div>
        <button
          type="button"
          onMouseDown={onBeforeDown}
          onMouseUp={onBeforeUp}
          onMouseLeave={onBeforeUp}
          onTouchStart={(e) => {
            e.preventDefault();
            onBeforeDown();
          }}
          onTouchEnd={onBeforeUp}
          aria-label={t("studio.editor.beforeHold", { defaultValue: "Hold to view before" })}
          className={`btn btn-sm gap-1.5 ${
            beforeActive ? "btn-neutral" : "btn-ghost text-base-content/70"
          }`}
        >
          {beforeActive ? <Eye size={15} /> : <EyeOff size={15} />}
          <span className="hidden md:inline">
            {t("studio.editor.before", { defaultValue: "Before" })}
          </span>
        </button>

        <div className="mx-0.5 h-6 w-px bg-base-300" />

        <button
          type="button"
          onClick={onSave}
          disabled={isSaving}
          className="btn btn-primary btn-sm gap-1.5"
        >
          {isSaving ? <Loader2 size={15} className="animate-spin" /> : <Save size={15} />}
          {t("studio.editor.save", { defaultValue: "Save" })}
        </button>
        <button
          type="button"
          onClick={onExport}
          disabled={isExporting}
          className="btn btn-sm gap-1.5 border-base-300 bg-base-100 text-base-content/80"
        >
          {isExporting ? <Loader2 size={15} className="animate-spin" /> : <Download size={15} />}
          <span className="hidden lg:inline">
            {t("studio.editor.export", { defaultValue: "Export" })}
          </span>
        </button>

        {/* Mobile: open the editor sheet. */}
        <button
          type="button"
          onClick={onOpenEdit}
          aria-label={t("studio.panel.edit", { defaultValue: "Editor" })}
          className="btn btn-ghost btn-sm btn-square text-base-content/70 lg:hidden"
        >
          <SlidersHorizontal size={17} />
        </button>

        {/* Desktop: collapse the editor rail. */}
        <div
          className="tooltip tooltip-bottom hidden lg:block"
          data-tip={t("studio.panel.edit", { defaultValue: "Editor panel" })}
        >
          <button
            type="button"
            onClick={onToggleRight}
            aria-pressed={rightOpen}
            aria-label={t("studio.panel.edit", { defaultValue: "Editor panel" })}
            className={`btn btn-ghost btn-sm btn-square ${
              rightOpen ? "text-base-content/80" : "text-base-content/40"
            }`}
          >
            <PanelRight size={17} />
          </button>
        </div>
      </div>
    </header>
  );
}
