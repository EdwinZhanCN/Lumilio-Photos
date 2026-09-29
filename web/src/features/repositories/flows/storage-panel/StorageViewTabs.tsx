import { useI18n } from "@/lib/i18n";

export type StorageViewMode = "browse" | "history";

type Props = {
  mode: StorageViewMode;
  onModeChange: (mode: StorageViewMode) => void;
};

/**
 * The storage page's two views. This is content, not page chrome: it sits at the
 * top of the scrolling content area rather than inside or against the page
 * header, so the header keeps only the title, the summary, and the create
 * action. The primary action lives in the header's action slot; Browse is a
 * plain reading surface with no search, filter, or sort.
 */
export default function StorageViewTabs({ mode, onModeChange }: Props) {
  const { t } = useI18n();

  return (
    <div role="tablist" className="tabs tabs-box tabs-sm self-start">
      <button
        type="button"
        role="tab"
        className={`tab ${mode === "browse" ? "tab-active" : ""}`}
        aria-selected={mode === "browse"}
        onClick={() => onModeChange("browse")}
      >
        {t("storagePanel.view.browse", "Browse")}
      </button>
      <button
        type="button"
        role="tab"
        className={`tab ${mode === "history" ? "tab-active" : ""}`}
        aria-selected={mode === "history"}
        onClick={() => onModeChange("history")}
      >
        {t("storagePanel.view.history", "History")}
      </button>
    </div>
  );
}
