import { useMemo, useState } from "react";
import {
  Check,
  Loader2,
  MoreVertical,
  Rocket,
  SquareMousePointer,
  Star,
  UserRoundCog,
  UserRoundX,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/features/notifications";
import { assetUrls } from "@/lib/assets/assetUrls";
import {
  useMoveFace,
  usePersonFaces,
  useRemoveFaceFromPerson,
  useSetPersonCover,
} from "../hooks/usePeople";
import PersonPicker from "./PersonPicker";

interface PersonFacesPanelProps {
  personId: number;
}

type PendingBulkAction = "move" | "remove" | null;

/**
 * Face-level correction surface for a person. Normal mode keeps per-face actions
 * local to each crop; selection mode exposes bulk reassignment/removal.
 * Corrections are entity actions on the person, so nothing here is
 * repository-scoped: all faces are shown regardless of where they live.
 */
export default function PersonFacesPanel({ personId }: PersonFacesPanelProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const { faces, total, isLoading } = usePersonFaces(personId);
  const { setPersonCover, isSettingCover } = useSetPersonCover();
  const { moveFace, isMoving } = useMoveFace();
  const { removeFace, isRemoving } = useRemoveFaceFromPerson();

  const [isSelectionMode, setIsSelectionMode] = useState(false);
  const [selectedFaceIds, setSelectedFaceIds] = useState<number[]>([]);
  const [pendingAction, setPendingAction] = useState<PendingBulkAction>(null);
  const [targetIds, setTargetIds] = useState<number[]>([]);

  const visibleFaceIds = useMemo(
    () => faces.map((face) => face.face_id ?? 0).filter((faceId) => faceId > 0),
    [faces],
  );
  const selectedFaceIdSet = useMemo(() => new Set(selectedFaceIds), [selectedFaceIds]);
  const allVisibleSelected =
    visibleFaceIds.length > 0 && visibleFaceIds.every((faceId) => selectedFaceIdSet.has(faceId));
  const busy = isSettingCover || isMoving || isRemoving;

  const enterSelectionMode = () => {
    setIsSelectionMode(true);
  };

  const exitSelectionMode = () => {
    setIsSelectionMode(false);
    setPendingAction(null);
    setTargetIds([]);
    setSelectedFaceIds([]);
  };

  const toggleFace = (faceId: number) => {
    if (!isSelectionMode) return;
    setPendingAction(null);
    setTargetIds([]);
    setSelectedFaceIds((current) =>
      current.includes(faceId) ? current.filter((id) => id !== faceId) : [...current, faceId],
    );
  };

  const selectAllVisible = () => {
    if (!isSelectionMode) return;
    setPendingAction(null);
    setTargetIds([]);
    setSelectedFaceIds(allVisibleSelected ? [] : visibleFaceIds);
  };

  const handleSetCover = async (faceId: number) => {
    if (!faceId || busy) return;
    try {
      await setPersonCover(personId, faceId);
      showMessage("success", t("people.cover.success", "Cover updated"));
    } catch (err) {
      showMessage(
        "error",
        t("people.cover.error", "Failed to set cover: {{message}}", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  const handleMoveSelected = async () => {
    const targetId = targetIds[0];
    if (!targetId || selectedFaceIds.length === 0 || busy) return;
    try {
      await Promise.all(selectedFaceIds.map((faceId) => moveFace(personId, faceId, targetId)));
      showMessage(
        "success",
        t("people.moveFace.bulkSuccess", "Moved {{count}} faces", {
          count: selectedFaceIds.length,
        }),
      );
      exitSelectionMode();
    } catch (err) {
      showMessage(
        "error",
        t("people.moveFace.bulkError", "Failed to move faces: {{message}}", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  const handleRemoveSelected = async () => {
    if (selectedFaceIds.length === 0 || busy) return;
    try {
      await Promise.all(selectedFaceIds.map((faceId) => removeFace(personId, faceId)));
      showMessage(
        "success",
        t("people.removeFace.bulkSuccess", "Removed {{count}} faces", {
          count: selectedFaceIds.length,
        }),
      );
      exitSelectionMode();
    } catch (err) {
      showMessage(
        "error",
        t("people.removeFace.bulkError", "Failed to remove faces: {{message}}", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  return (
    <section className="space-y-3">
      <header className="relative z-40 flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">{t("people.faces.title", "Faces")}</h2>
          {!isLoading && (
            <span className="text-xs text-base-content/55">
              {t("people.faces.count", "{{count}} faces", { count: total })}
            </span>
          )}
        </div>

        {!isLoading && faces.length > 0 && (
          <>
            {isSelectionMode ? (
              <div className="flex flex-wrap items-center gap-2">
                <span className="badge badge-neutral badge-sm rounded-full px-3">
                  {t("people.faces.selectedCount", "{{count}} selected", {
                    count: selectedFaceIds.length,
                  })}
                </span>
                <button type="button" className="btn btn-ghost btn-sm" onClick={selectAllVisible}>
                  {allVisibleSelected
                    ? t("people.faces.clearSelection", "Clear")
                    : t("people.faces.selectAll", "Select all")}
                </button>
                <details className="dropdown dropdown-end z-50">
                  <summary
                    className={`btn btn-soft btn-accent btn-sm gap-2 rounded-full ${
                      selectedFaceIds.length === 0 || busy ? "btn-disabled opacity-50" : ""
                    }`}
                    aria-disabled={selectedFaceIds.length === 0 || busy}
                    onClick={(event) => {
                      if (selectedFaceIds.length === 0 || busy) event.preventDefault();
                    }}
                  >
                    {busy ? (
                      <span className="loading loading-spinner loading-xs" />
                    ) : (
                      <Rocket className="size-4" />
                    )}
                    {t("people.faces.actions", "Actions")}
                  </summary>
                  <ul className="menu dropdown-content z-[80] mt-2 w-60 rounded-box border border-base-200 bg-base-100 p-2 shadow-xl">
                    <li className="menu-title px-3 py-1 text-xs text-base-content/45">
                      {t("people.faces.selectedCount", "{{count}} selected", {
                        count: selectedFaceIds.length,
                      })}
                    </li>
                    <li>
                      <button
                        type="button"
                        disabled={selectedFaceIds.length === 0 || busy}
                        onClick={() => {
                          setPendingAction("move");
                          setTargetIds([]);
                        }}
                      >
                        <UserRoundCog className="size-4" />
                        {t("people.faces.move", "Move to another person")}
                      </button>
                    </li>
                    <li>
                      <button
                        type="button"
                        className="text-error"
                        disabled={selectedFaceIds.length === 0 || busy}
                        onClick={() => setPendingAction("remove")}
                      >
                        <UserRoundX className="size-4" />
                        {t("people.faces.remove", "Remove from this person")}
                      </button>
                    </li>
                  </ul>
                </details>
                <button type="button" className="btn btn-ghost btn-sm" onClick={exitSelectionMode}>
                  {t("people.faces.selectionMode.exit", "Done")}
                </button>
              </div>
            ) : (
              <button
                type="button"
                className="btn btn-soft btn-info btn-sm gap-2 rounded-full"
                onClick={enterSelectionMode}
              >
                <SquareMousePointer className="size-4" />
                {t("people.faces.selectionMode.enter", "Select")}
              </button>
            )}
          </>
        )}
      </header>

      {pendingAction === "move" && (
        <div className="space-y-3 rounded-box border border-base-300 bg-base-200/35 p-3">
          <div>
            <h3 className="text-sm font-semibold">
              {t("people.moveFace.bulkTitle", "Move selected faces")}
            </h3>
            <p className="text-xs text-base-content/60">
              {t("people.moveFace.bulkDescription", "Choose the person these faces belong to.")}
            </p>
          </div>
          <PersonPicker excludeIds={[personId]} selectedIds={targetIds} onChange={setTargetIds} />
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={() => setPendingAction(null)}
            >
              {t("common.cancel", "Cancel")}
            </button>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={handleMoveSelected}
              disabled={!targetIds[0] || busy}
            >
              {isMoving && <span className="loading loading-spinner loading-xs" />}
              {t("people.moveFace.bulkConfirm", "Move selected")}
            </button>
          </div>
        </div>
      )}

      {pendingAction === "remove" && (
        <div className="space-y-3 rounded-box border border-error/30 bg-error/5 p-3">
          <div>
            <h3 className="text-sm font-semibold text-error">
              {t("people.removeFace.bulkTitle", "Remove selected faces")}
            </h3>
            <p className="text-xs text-base-content/70">
              {t(
                "people.removeFace.bulkDescription",
                "The selected faces will no longer be associated with this person. Original photos are not changed.",
              )}
            </p>
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={() => setPendingAction(null)}
            >
              {t("common.cancel", "Cancel")}
            </button>
            <button
              type="button"
              className="btn btn-error btn-sm"
              onClick={handleRemoveSelected}
              disabled={busy}
            >
              {isRemoving && <span className="loading loading-spinner loading-xs" />}
              {t("people.removeFace.bulkConfirm", "Remove selected")}
            </button>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-8">
          <Loader2 className="size-6 animate-spin text-base-content/40" />
        </div>
      ) : faces.length === 0 ? (
        <div className="rounded-box border border-dashed border-base-300 px-5 py-6 text-sm text-base-content/60">
          {t("people.faces.empty", "No detected faces for this person.")}
        </div>
      ) : (
        <div className="grid grid-cols-4 gap-2 sm:grid-cols-5 md:grid-cols-7 lg:grid-cols-8">
          {faces.map((face) => {
            const faceId = face.face_id ?? 0;
            const cropUrl = face.has_crop ? assetUrls.getFaceCropUrl(personId, faceId) : null;
            const selected = selectedFaceIdSet.has(faceId);
            return (
              <div key={faceId} className="group relative">
                {isSelectionMode && (
                  <input
                    type="checkbox"
                    className="peer sr-only"
                    checked={selected}
                    onChange={() => toggleFace(faceId)}
                    disabled={faceId <= 0 || busy}
                  />
                )}
                <div
                  role={isSelectionMode ? "button" : undefined}
                  tabIndex={isSelectionMode ? 0 : undefined}
                  onClick={() => toggleFace(faceId)}
                  onKeyDown={(event) => {
                    if (!isSelectionMode) return;
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      toggleFace(faceId);
                    }
                  }}
                  className={`relative aspect-square overflow-hidden rounded-xl border border-base-100/10 bg-base-200/60 transition-all duration-200 ${
                    selected
                      ? "ring-2 ring-primary/80 ring-inset shadow-[0_20px_40px_-26px_rgba(59,130,246,0.55)]"
                      : isSelectionMode
                        ? "cursor-pointer ring-1 ring-black/10 ring-inset"
                        : "shadow-[0_16px_40px_-30px_rgba(15,23,42,0.45)] group-hover:-translate-y-0.5"
                  }`}
                >
                  {isSelectionMode && (
                    <>
                      <div
                        className={`pointer-events-none absolute inset-0 z-10 transition-colors ${
                          selected
                            ? "bg-primary/14"
                            : "bg-gradient-to-b from-black/20 via-transparent to-black/10"
                        }`}
                      />
                      <div className="absolute right-2 top-2 z-20">
                        <div
                          className={`flex size-7 items-center justify-center rounded-full border backdrop-blur-md transition-all ${
                            selected
                              ? "border-primary/70 bg-primary text-primary-content shadow-lg shadow-primary/25"
                              : "border-white/30 bg-black/35 text-white/75 shadow-lg shadow-black/20"
                          }`}
                        >
                          {selected ? (
                            <Check className="size-4" strokeWidth={3} />
                          ) : (
                            <div className="size-3 rounded-full border border-current/80" />
                          )}
                        </div>
                      </div>
                    </>
                  )}
                  {cropUrl ? (
                    <img
                      src={cropUrl}
                      alt=""
                      className={`size-full object-cover transition-transform duration-300 ${
                        isSelectionMode || selected ? "" : "group-hover:scale-[1.03]"
                      }`}
                      loading="lazy"
                    />
                  ) : (
                    <div className="flex size-full items-center justify-center text-xs text-base-content/40">
                      {t("people.faces.missingCrop", "No crop")}
                    </div>
                  )}
                </div>

                {face.is_representative && (
                  <span className="badge badge-primary badge-xs absolute left-2 top-2 z-20 gap-1 border-none">
                    <Star className="size-3" />
                  </span>
                )}

                {!isSelectionMode && (
                  <details className="dropdown dropdown-end absolute right-1.5 top-1.5 z-30">
                    <summary className="btn btn-circle btn-ghost btn-xs bg-base-100/80 shadow-sm backdrop-blur">
                      <MoreVertical className="size-4" />
                    </summary>
                    <ul className="menu dropdown-content z-10 mt-1 w-44 rounded-box border border-base-200 bg-base-100 p-2 shadow-xl">
                      <li>
                        <button
                          type="button"
                          disabled={face.is_representative || busy}
                          onClick={() => void handleSetCover(faceId)}
                        >
                          <Star className="size-4" />
                          {t("people.faces.setCover", "Set as cover")}
                        </button>
                      </li>
                    </ul>
                  </details>
                )}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
