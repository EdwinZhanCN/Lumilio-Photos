import React from "react";
import { Check, Cpu } from "lucide-react";
import { useI18n } from "@/lib/i18n";

type StatusBarProps = {
  dirty: boolean;
  justSaved: boolean;
  engine?: string;
};

/**
 * A thin footer for ambient status that does not belong in the action-dense top
 * bar: the save state and the active render backend.
 */
export function StatusBar({ dirty, justSaved, engine }: StatusBarProps): React.JSX.Element {
  const { t } = useI18n();

  return (
    <footer className="flex h-7 shrink-0 items-center justify-between border-t border-base-300 bg-base-100 px-3 text-[11px] text-base-content/50">
      <div className="flex items-center gap-1.5">
        {justSaved ? (
          <span className="flex items-center gap-1 font-medium text-success">
            <Check size={12} />
            {t("studio.editor.saved", { defaultValue: "Saved" })}
          </span>
        ) : dirty ? (
          <span className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-warning" />
            {t("studio.editor.unsaved", { defaultValue: "Unsaved changes" })}
          </span>
        ) : (
          <span>{t("studio.editor.upToDate", { defaultValue: "Up to date" })}</span>
        )}
      </div>

      {engine && (
        <span className="flex items-center gap-1 font-mono text-base-content/45">
          <Cpu size={11} />
          {engine}
        </span>
      )}
    </footer>
  );
}
