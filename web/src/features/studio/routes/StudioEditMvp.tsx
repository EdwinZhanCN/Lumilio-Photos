import React, { useCallback, useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import PhotoPicker from "@/features/assets/picker";
import { useI18n } from "@/lib/i18n";
import { StudioHome } from "../modules/home/StudioHome";
import { StudioEditor, type StudioEditorActivity } from "../modules/editor/StudioEditor";
import {
  clearRecentEdits,
  readRecentEdits,
  recordRecentEdit,
  type RecentEditRecord,
} from "../state/recentEdits";

type StudioView = "home" | "picker" | "editor";

/**
 * Studio shell: a single `/studio` route that switches between the Home
 * dashboard, a photo picker, and the develop editor. The app already provides
 * global navigation, so Studio deliberately does not render its own nav rail.
 */
export function StudioEditMvp(): React.JSX.Element {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const [view, setView] = useState<StudioView>("home");
  const [assetId, setAssetId] = useState<string | null>(() => searchParams.get("assetId"));
  const [focusBorder, setFocusBorder] = useState(false);
  const [recent, setRecent] = useState<RecentEditRecord[]>(() => readRecentEdits());

  // If provided via search param, skip to editor
  useEffect(() => {
    const urlAssetId = searchParams.get("assetId");
    if (urlAssetId) {
      setAssetId(urlAssetId);
      setView("editor");
    }
  }, [searchParams]);

  const openPicker = useCallback((withBorder: boolean) => {
    setFocusBorder(withBorder);
    setView("picker");
  }, []);

  const resume = useCallback((id: string) => {
    setFocusBorder(false);
    setAssetId(id);
    setView("editor");
  }, []);

  const handlePicked = useCallback((id: string) => {
    setAssetId(id);
    setView("editor");
  }, []);

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

  if (view === "editor" && assetId) {
    return (
      <div className="h-full overflow-hidden bg-base-100">
        <StudioEditor
          key={assetId}
          assetId={assetId}
          focusBorder={focusBorder}
          onBack={() => setView("home")}
          onActivity={handleActivity}
        />
      </div>
    );
  }

  if (view === "picker") {
    return (
      <div className="flex h-full flex-col overflow-hidden bg-base-100">
        <div className="flex h-12 shrink-0 items-center border-b border-base-300 px-3">
          <button
            type="button"
            onClick={() => setView("home")}
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
        onOpenEditor={() => openPicker(false)}
        onOpenBorderTool={() => openPicker(true)}
        onResume={resume}
        onClearRecent={handleClearRecent}
      />
    </div>
  );
}
