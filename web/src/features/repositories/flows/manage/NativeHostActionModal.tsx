import { useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, FolderOpen, HardDrive, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem, localizeProblemReference } from "@/lib/http-commons/problem";
import { createUUID } from "@/lib/uuid";
import {
  type HostAction,
  type HostActionKind,
  type HostActionResolution,
  useCancelNativeHostAction,
  useCreateNativeHostAction,
  useNativeHostAction,
  useResolveNativeHostAction,
  useUnfinishedNativeHostActions,
} from "../../api/useNativeHostActions";
import {
  clearNativeHostActionID,
  loadNativeHostActionID,
  saveNativeHostActionID,
} from "../../state/nativeHostActionState";

const terminalStatuses = new Set(["succeeded", "failed", "cancelled", "expired"]);

export default function NativeHostActionModal({
  isOpen,
  kind,
  rootId,
  repositoryId,
  onClose,
}: {
  isOpen: boolean;
  kind: HostActionKind;
  rootId?: string;
  repositoryId?: string;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const createMutation = useCreateNativeHostAction();
  const resolveMutation = useResolveNativeHostAction();
  const cancelMutation = useCancelNativeHostAction();
  const [action, setAction] = useState<HostAction | null>(null);
  const [name, setName] = useState("");
  const [confirmSeparate, setConfirmSeparate] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const requestID = useRef(createUUID());
  const persistenceScope = rootId || repositoryId || "new";
  const shouldPoll = Boolean(action?.id && !terminalStatuses.has(action.status ?? ""));
  const actionQuery = useNativeHostAction(action?.id ?? "", shouldPoll);
  const unfinishedQuery = useUnfinishedNativeHostActions(isOpen && !action);
  const busy = createMutation.isPending || resolveMutation.isPending || cancelMutation.isPending;

  useEffect(() => {
    if (actionQuery.data) setAction(actionQuery.data);
  }, [actionQuery.data]);

  useEffect(() => {
    if (!isOpen || action) return;
    const savedActionID = loadNativeHostActionID(kind, persistenceScope);
    if (savedActionID) {
      setAction({ id: savedActionID, status: "pending" } as HostAction);
    }
  }, [action, isOpen, kind, persistenceScope]);

  useEffect(() => {
    if (!isOpen || action || !unfinishedQuery.data) return;
    const recovered = unfinishedQuery.data.find(
      (candidate) =>
        candidate.kind === kind &&
        (candidate.root_id ?? "") === (rootId ?? "") &&
        (candidate.repository_id ?? "") === (repositoryId ?? ""),
    );
    if (recovered) setAction(recovered);
  }, [action, isOpen, kind, repositoryId, rootId, unfinishedQuery.data]);

  useEffect(() => {
    if (!action?.id) return;
    if (terminalStatuses.has(action.status ?? "")) {
      clearNativeHostActionID(kind, persistenceScope);
      return;
    }
    saveNativeHostActionID(kind, persistenceScope, action.id);
  }, [action?.id, action?.status, kind, persistenceScope]);

  useEffect(() => {
    if (action?.status !== "succeeded") return;
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repository-roots"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/repositories"] }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/indexing/repositories"] }),
    ]);
  }, [action?.status, queryClient]);

  useEffect(() => {
    if (isOpen) return;
    setAction(null);
    setName("");
    setConfirmSeparate(false);
    setError(null);
    requestID.current = createUUID();
  }, [isOpen]);

  const title = hostActionTitle(kind, t);
  const purpose = hostActionPurpose(kind, t);
  const Icon =
    kind === "authorize_storage_location" || kind === "locate_storage_location"
      ? HardDrive
      : FolderOpen;
  const conflict = action?.result?.conflict;
  const allowedResolutions = useMemo(
    () => new Set(conflict?.allowed_resolutions ?? []),
    [conflict?.allowed_resolutions],
  );
  const status = action?.status ?? "";
  const stepIndex = !action ? 0 : terminalStatuses.has(status) ? 2 : 1;
  const endedWithFailure =
    Boolean(action) && (status === "failed" || status === "cancelled" || status === "expired");
  const stepLabels = [
    t("manage.repositories.hostAction.stepSend", "Send request"),
    t("manage.repositories.hostAction.stepApprove", "Desktop approval"),
    t("manage.repositories.hostAction.stepResult", "Result"),
  ];

  const submit = async () => {
    setError(null);
    try {
      const created = await createMutation.mutateAsync({
        params: { header: { "Idempotency-Key": requestID.current } },
        body: {
          kind,
          name: kind === "authorize_storage_location" ? name.trim() : undefined,
          purpose,
          root_id: rootId,
          repository_id: repositoryId,
          session_id: createUUID(),
          expires_in_seconds: 600,
        },
      });
      if (created.id) saveNativeHostActionID(kind, persistenceScope, created.id);
      setAction(created);
    } catch (reason: unknown) {
      setError(
        actionErrorMessage(
          reason,
          t,
          t(
            "manage.repositories.hostAction.createFailed",
            "The Desktop request could not be created.",
          ),
        ),
      );
    }
  };

  const resolve = async (resolution: HostActionResolution) => {
    if (!action?.id) return;
    setError(null);
    try {
      const resolved = await resolveMutation.mutateAsync({
        params: { path: { id: action.id } },
        body: { resolution },
      });
      setAction(resolved);
    } catch (reason: unknown) {
      setError(
        actionErrorMessage(
          reason,
          t,
          t(
            "manage.repositories.hostAction.resolveFailed",
            "The recovery decision could not be applied.",
          ),
        ),
      );
    }
  };

  const cancel = async () => {
    if (!action?.id) {
      onClose();
      return;
    }
    setError(null);
    try {
      await cancelMutation.mutateAsync({ params: { path: { id: action.id } } });
      onClose();
    } catch (reason: unknown) {
      setError(
        actionErrorMessage(
          reason,
          t,
          t("manage.repositories.hostAction.cancelFailed", "The request could not be cancelled."),
        ),
      );
    }
  };

  if (!isOpen) return null;

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-lg">
        <div className="mb-5 flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Icon size={20} />
            </div>
            <div>
              <h3 className="text-base font-semibold">{title}</h3>
              <p className="mt-0.5 text-sm text-base-content/60">{purpose}</p>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={() => void cancel()}
            disabled={busy}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        <ul className="steps steps-horizontal mb-5 w-full text-xs">
          {stepLabels.map((label, index) => {
            let stepClass = "step";
            if (index < stepIndex) stepClass += " step-success";
            else if (index === stepIndex) {
              if (status === "succeeded") stepClass += " step-success";
              else stepClass += endedWithFailure ? " step-error" : " step-primary";
            }
            return (
              <li key={label} className={stepClass}>
                {label}
              </li>
            );
          })}
        </ul>

        {!action ? (
          <div className="space-y-4">
            {kind === "authorize_storage_location" ? (
              <label className="fieldset w-full gap-1">
                <span className="fieldset-legend p-0 text-sm font-medium">
                  {t("manage.repositories.hostAction.locationName", "Location name")}
                </span>
                <input
                  className="input input-bordered w-full"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={t(
                    "manage.repositories.hostAction.locationNamePlaceholder",
                    "External Archive",
                  )}
                  autoFocus
                />
              </label>
            ) : null}
            <div role="alert" className="alert alert-info alert-soft text-sm">
              {t(
                "manage.repositories.hostAction.desktopApproval",
                "After you send this request, review it in Desktop and choose the folder on that computer. The Web app never submits a filesystem path.",
              )}
            </div>
            <div className="modal-action">
              <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
                {t("common.cancel")}
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => void submit()}
                disabled={busy}
              >
                {createMutation.isPending ? (
                  <span className="loading loading-spinner loading-sm" />
                ) : null}
                {t("manage.repositories.hostAction.send", "Send to Desktop")}
              </button>
            </div>
          </div>
        ) : action.status === "pending" || action.status === "running" ? (
          <div className="space-y-4">
            <div className="flex min-h-32 flex-col items-center justify-center gap-3 rounded-lg border border-base-300 bg-base-200/30 px-6 text-center">
              <span className="loading loading-spinner loading-md text-primary" />
              <strong>
                {t("manage.repositories.hostAction.waiting", "Waiting for Desktop approval")}
              </strong>
              <p className="max-w-sm text-sm text-base-content/60">
                {t(
                  "manage.repositories.hostAction.waitingHint",
                  "Open Desktop Settings → Storage, review this request, then choose a folder locally.",
                )}
              </p>
              <p className="text-xs text-base-content/50">
                {t("manage.repositories.hostAction.requestedBy", "Requested by {{actor}}", {
                  actor:
                    action.actor ||
                    t("manage.repositories.hostAction.unknownActor", "Unknown administrator"),
                })}
              </p>
            </div>
            <div className="modal-action">
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => void cancel()}
                disabled={busy}
              >
                {t("common.cancel")}
              </button>
            </div>
          </div>
        ) : action.status === "needs_decision" && conflict ? (
          <div className="space-y-4">
            <div role="alert" className="alert alert-warning alert-soft items-start text-sm">
              <AlertTriangle className="mt-0.5 size-5 shrink-0" />
              <div>
                <strong>
                  {t(
                    "manage.repositories.hostAction.identityConflict",
                    "This identity is already registered",
                  )}
                </strong>
                <p className="mt-1">
                  {t(
                    "manage.repositories.hostAction.identityConflictHint",
                    "Choose whether the selected folder is the moved original or an independent copy. No files will be deleted.",
                  )}
                </p>
              </div>
            </div>
            {allowedResolutions.has("add_separate") ? (
              <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-base-300 p-3 text-sm">
                <input
                  type="checkbox"
                  className="checkbox checkbox-sm mt-0.5"
                  checked={confirmSeparate}
                  onChange={(event) => setConfirmSeparate(event.target.checked)}
                />
                <span>
                  {t(
                    "manage.repositories.hostAction.copyConfirmation",
                    "I understand this creates a separate Repository identity and isolates copied private state before scanning.",
                  )}
                </span>
              </label>
            ) : null}
            <div className="modal-action flex-wrap">
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => void cancel()}
                disabled={busy}
              >
                {t("common.cancel")}
              </button>
              {allowedResolutions.has("update_location") ? (
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => void resolve("update_location")}
                  disabled={busy}
                >
                  {t("manage.repositories.hostAction.updateLocation", "Use as moved original")}
                </button>
              ) : null}
              {allowedResolutions.has("add_separate") ? (
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={() => void resolve("add_separate")}
                  disabled={busy || !confirmSeparate}
                >
                  {t("manage.repositories.hostAction.addSeparate", "Add as separate Repository")}
                </button>
              ) : null}
            </div>
          </div>
        ) : action.status === "succeeded" ? (
          <div className="space-y-4">
            <div className="flex min-h-32 flex-col items-center justify-center gap-3 rounded-lg border border-success/30 bg-success/10 px-6 text-center text-success">
              <CheckCircle2 size={28} />
              <strong>
                {t("manage.repositories.hostAction.completed", "Desktop request completed")}
              </strong>
              <p className="text-sm opacity-80">{action.result?.name || title}</p>
            </div>
            <div className="modal-action">
              <button type="button" className="btn btn-primary" onClick={onClose}>
                {t("common.done", { defaultValue: "Done" })}
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <div role="alert" className="alert alert-error alert-soft text-sm">
              <AlertTriangle className="size-5 shrink-0" />
              <span>
                {localizeProblemReference(
                  action.problem,
                  t,
                  t(
                    "manage.repositories.hostAction.ended",
                    "The Desktop request ended before it completed.",
                  ),
                )}
              </span>
            </div>
            <div className="modal-action">
              <button type="button" className="btn btn-primary" onClick={onClose}>
                {t("common.close", { defaultValue: "Close" })}
              </button>
            </div>
          </div>
        )}

        {error ? (
          <div role="alert" className="alert alert-error alert-soft mt-4 py-2 text-sm">
            {error}
          </div>
        ) : null}
      </div>
      <button
        type="button"
        className="modal-backdrop"
        aria-label={t("common.close", { defaultValue: "Close" })}
        onClick={() => void cancel()}
        disabled={busy}
      />
    </div>
  );
}

function actionErrorMessage(
  reason: unknown,
  t: ReturnType<typeof useI18n>["t"],
  fallback: string,
): string {
  return localizeAPIProblem(reason, t, fallback);
}

function hostActionTitle(kind: HostActionKind, t: ReturnType<typeof useI18n>["t"]): string {
  switch (kind) {
    case "authorize_storage_location":
      return t("manage.repositories.hostAction.addLocation", "Add Storage Location");
    case "locate_storage_location":
      return t("manage.repositories.hostAction.locateLocation", "Reconnect Storage Location");
    case "locate_repository":
      return t("manage.repositories.hostAction.locateRepository", "Locate Repository");
    default:
      return t("manage.repositories.hostAction.openRepository", "Open Existing Repository");
  }
}

function hostActionPurpose(kind: HostActionKind, t: ReturnType<typeof useI18n>["t"]): string {
  switch (kind) {
    case "authorize_storage_location":
      return t(
        "manage.repositories.hostAction.addPurpose",
        "Choose and authorize a folder for Repositories",
      );
    case "locate_storage_location":
      return t(
        "manage.repositories.hostAction.locateLocationPurpose",
        "Choose the current folder for this moved or reconnected Storage Location",
      );
    case "locate_repository":
      return t(
        "manage.repositories.hostAction.locateRepositoryPurpose",
        "Choose the current folder for this moved Repository",
      );
    default:
      return t(
        "manage.repositories.hostAction.openPurpose",
        "Choose a folder that contains an existing Repository",
      );
  }
}
