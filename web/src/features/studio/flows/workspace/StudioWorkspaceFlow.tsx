import React, { useCallback, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import PhotoPicker from "@/features/assets/picker";
import { useI18n } from "@/lib/i18n";
import { StudioHome } from "../home/StudioHome";
import { StudioEditor, type StudioEditorActivity } from "../editor/StudioEditor";
import type { EditorTab } from "../editor/EditorPanel";
import {
  clearRecentEdits,
  readRecentEdits,
  recordRecentEdit,
  type RecentEditRecord,
} from "../../state/recentEdits";

/**
 * Studio shell: a single `/studio` route that switches between the Home
 * dashboard, a photo picker, and the develop editor. The app already provides
 * global navigation, so Studio deliberately does not render its own nav rail.
 *
 * The open photo is owned by the URL (`?assetId=`), so a refresh or a shared
 * link restores the editor instead of dropping back to Home. The picker is a
 * transient sub-view and stays local.
 */
export function StudioEditMvp(): React.JSX.Element {
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const assetId = searchParams.get("assetId");
  const [showPicker, setShowPicker] = useState(false);
  const [focusTab, setFocusTab] = useState<EditorTab>("develop");
  const [recent, setRecent] = useState<RecentEditRecord[]>(() => readRecentEdits());

  const openEditor = useCallback(
    (id: string, tab: EditorTab) => {
      setFocusTab(tab);
      setShowPicker(false);
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set("assetId", id);
        return next;
      });
    },
    [setSearchParams],
  );

  const closeEditor = useCallback(() => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.delete("assetId");
      return next;
    });
  }, [setSearchParams]);

  // Pick a photo, then open the editor on the chosen tool.
  const openPicker = useCallback((tab: EditorTab) => {
    setFocusTab(tab);
    setShowPicker(true);
  }, []);

  const resume = useCallback((id: string) => openEditor(id, "develop"), [openEditor]);
  const handlePicked = useCallback((id: string) => openEditor(id, focusTab), [openEditor, focusTab]);

  const handleActivity = useCallback((activity: StudioEditorActivity) => {
    setRecent(
      recordRecentEdit({
        assetId: activity.assetId,
        name: activity.name,
        width: activity.width,
        height: activity.height,
      }),
    );
  }, []);

  const handleClearRecent = useCallback(() => {
    clearRecentEdits();
    setRecent([]);
  }, []);

  if (assetId) {
    return (
      <div className="h-full overflow-hidden bg-base-100">
        <StudioEditor
          key={assetId}
          assetId={assetId}
          initialTab={focusTab}
          onBack={closeEditor}
          onActivity={handleActivity}
        />
      </div>
    );
  }

  if (showPicker) {
    return (
      <div className="flex h-full flex-col overflow-hidden bg-base-100">
        <div className="flex h-12 shrink-0 items-center border-b border-base-300 px-3">
          <button
            type="button"
            onClick={() => setShowPicker(false)}
            className="btn btn-ghost btn-sm gap-2 text-base-content/80"
          >
            <ArrowLeft size={16} />
            {t("studio.backToStudio", { defaultValue: "Back to Studio" })}
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-hidden">
          <PhotoPicker
            scopeId="studio:editor"
            title={t("studio.pickPhoto", {
              defaultValue: "Pick a photo to edit",
            })}
            initialFilters={{ raw: false }}
            lockedFields={["type", "raw"]}
            onSelect={handlePicked}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-hidden bg-base-100">
      <StudioHome
        recent={recent}
        onOpenTool={openPicker}
        onResume={resume}
        onClearRecent={handleClearRecent}
      />
    </div>
  );
}
