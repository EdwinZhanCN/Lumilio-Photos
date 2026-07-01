import React from "react";
import {
  ArrowLeft,
  Check,
  Cpu,
  Download,
  Eye,
  EyeOff,
  Loader2,
  RotateCw,
  Save,
  Undo2,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";

type TopBarProps = {
  fileName: string;
  engine?: string;
  dirty: boolean;
  justSaved: boolean;
  canUndo: boolean;
  beforeActive: boolean;
  isSaving: boolean;
  isExporting: boolean;
  onBack: () => void;
  onUndo: () => void;
  onResetAll: () => void;
  onBeforeDown: () => void;
  onBeforeUp: () => void;
  onSave: () => void;
  onExport: () => void;
};

export function TopBar({
  fileName,
  engine,
  dirty,
  justSaved,
  canUndo,
  beforeActive,
  isSaving,
  isExporting,
  onBack,
  onUndo,
  onResetAll,
  onBeforeDown,
  onBeforeUp,
  onSave,
  onExport,
}: TopBarProps): React.JSX.Element {
  const { t } = useI18n();

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-base-300 bg-base-100 px-3">
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

      <div className="h-6 w-px bg-base-300" />

      <div className="flex min-w-0 items-center gap-2.5">
        <span className="truncate text-sm font-medium text-base-content">{fileName}</span>
        {justSaved ? (
          <span className="flex items-center gap-1 text-xs font-medium text-success">
            <Check size={13} />
            {t("studio.editor.saved", { defaultValue: "Saved" })}
          </span>
        ) : dirty ? (
          <span className="flex items-center gap-1.5 text-xs text-base-content/50">
            <span className="h-1.5 w-1.5 rounded-full bg-warning" />
            {t("studio.editor.unsaved", { defaultValue: "Unsaved changes" })}
          </span>
        ) : (
          <span className="text-xs text-base-content/40">
            {t("studio.editor.upToDate", { defaultValue: "Up to date" })}
          </span>
        )}
        {engine && (
          <span className="badge badge-sm gap-1 border-base-300 bg-base-200 font-mono text-[10px] text-base-content/60">
            <Cpu size={10} />
            {engine}
          </span>
        )}
      </div>

      <div className="ml-auto flex items-center gap-1.5">
        <div
          className="tooltip tooltip-bottom"
          data-tip={t("studio.editor.undo", { defaultValue: "Undo" })}
        >
          <button
            type="button"
            onClick={onUndo}
            disabled={!canUndo}
            aria-label="Undo"
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
            aria-label="Reset all"
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
          aria-label="Hold to view before"
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
      </div>
    </header>
  );
}
