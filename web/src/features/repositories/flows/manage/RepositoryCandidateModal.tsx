import { useState } from "react";
import { AlertTriangle, Copy, FolderOpen, RefreshCw, X } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import {
  type RepositoryCandidate,
  useOpenRepositoryCandidate,
  useRepositoryCandidates,
  useResolveRepositoryCandidate,
} from "../../api/useRepositoryCandidates";

export default function RepositoryCandidateModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const candidatesQuery = useRepositoryCandidates(isOpen);
  const openMutation = useOpenRepositoryCandidate();
  const resolveMutation = useResolveRepositoryCandidate();
  const candidates = candidatesQuery.data?.candidates ?? [];
  const busy = openMutation.isPending || resolveMutation.isPending;
  const [riskConfirmations, setRiskConfirmations] = useState<Record<string, boolean>>({});
  const [separateArmedFor, setSeparateArmedFor] = useState<string | null>(null);

  const open = async (candidate: RepositoryCandidate) => {
    if (!candidate.can_open || !candidate.directory_name) return;
    try {
      await openMutation.openRepositoryCandidate(
        candidate.directory_name,
        riskConfirmations[candidate.directory_name] ?? false,
      );
      showMessage(
        "success",
        t(
          "manage.repositories.candidates.opened",
          "Repository opened and its initial scan was queued.",
        ),
      );
      onClose();
    } catch (reason: unknown) {
      showMessage(
        "error",
        localizeAPIProblem(
          reason,
          t,
          t("manage.repositories.candidates.openFailed", "Repository could not be opened."),
        ),
      );
    }
  };

  const resolve = async (
    candidate: RepositoryCandidate,
    resolution: "update_location" | "add_separate",
  ) => {
    if (!candidate.directory_name) return;
    try {
      await resolveMutation.resolveRepositoryCandidate(
        candidate.directory_name,
        resolution,
        riskConfirmations[candidate.directory_name] ?? false,
      );
      showMessage(
        "success",
        resolution === "update_location"
          ? t("manage.repositories.candidates.relocated", "Repository location updated.")
          : t("manage.repositories.candidates.copyAdded", "Separate Repository added."),
      );
      onClose();
    } catch (reason: unknown) {
      showMessage(
        "error",
        localizeAPIProblem(
          reason,
          t,
          t(
            "manage.repositories.candidates.resolveFailed",
            "Repository decision could not be applied.",
          ),
        ),
      );
    }
  };

  if (!isOpen) return null;

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-2xl">
        <div className="mb-5 flex items-start justify-between gap-4">
          <div>
            <h3 className="text-base font-semibold">
              {t("manage.repositories.candidates.title", "Open Existing Repository")}
            </h3>
            <p className="mt-1 text-sm text-base-content/60">
              {t(
                "manage.repositories.candidates.description",
                "Choose a direct child of the configured default Storage Location. Arbitrary server paths are not accepted.",
              )}
            </p>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            disabled={busy}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        {candidatesQuery.isLoading ? (
          <div className="flex min-h-32 items-center justify-center">
            <span className="loading loading-spinner loading-md" />
          </div>
        ) : candidatesQuery.isError ? (
          <div role="alert" className="alert alert-error alert-soft text-sm">
            <span>
              {t(
                "manage.repositories.candidates.unavailable",
                "Repository candidates are unavailable.",
              )}
            </span>
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => void candidatesQuery.refetch()}
            >
              <RefreshCw size={15} /> {t("common.retry")}
            </button>
          </div>
        ) : candidates.length === 0 ? (
          <div className="rounded-lg border border-base-300 px-5 py-8 text-center text-sm text-base-content/60">
            {t(
              "manage.repositories.candidates.empty",
              "No direct-child folders were found in the default Storage Location.",
            )}
          </div>
        ) : (
          <div className="max-h-[55vh] space-y-2 overflow-y-auto pr-1">
            {candidates.map((candidate) => {
              const armed = separateArmedFor === candidate.directory_name;
              return (
                <div
                  key={candidate.directory_name}
                  className="rounded-lg border border-base-300 px-4 py-3"
                >
                  <div className="flex items-center justify-between gap-4">
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-medium">
                        {candidate.name || candidate.directory_name}
                      </div>
                      <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-base-content/55">
                        <span className="font-mono">{candidate.directory_name}</span>
                        <span
                          className={`badge badge-sm ${candidateBadgeClass(candidate.classification)}`}
                        >
                          {candidateLabel(candidate.classification, t)}
                        </span>
                      </div>
                      {(candidate.risk_warnings?.length ?? 0) > 0 ? (
                        <label className="mt-2 flex items-start gap-2 text-xs text-warning">
                          <input
                            type="checkbox"
                            className="checkbox checkbox-warning checkbox-xs mt-0.5"
                            checked={riskConfirmations[candidate.directory_name ?? ""] ?? false}
                            onChange={(event) =>
                              setRiskConfirmations((current) => ({
                                ...current,
                                [candidate.directory_name ?? ""]: event.target.checked,
                              }))
                            }
                          />
                          <span>
                            <AlertTriangle className="mr-1 inline size-3" />
                            {t("manage.repositories.candidates.riskConfirmation", {
                              defaultValue:
                                "I understand this location may be network-backed, removable, managed by cloud-sync software, mounted differently than when it was registered, or contain files that must be downloaded first.",
                            })}{" "}
                            ({candidate.risk_warnings?.join(", ")})
                          </span>
                        </label>
                      ) : null}
                    </div>
                    <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
                      {candidate.can_open ? (
                        <button
                          type="button"
                          className="btn btn-primary btn-sm"
                          onClick={() => void open(candidate)}
                          disabled={
                            busy ||
                            ((candidate.risk_warnings?.length ?? 0) > 0 &&
                              !riskConfirmations[candidate.directory_name ?? ""])
                          }
                        >
                          {busy ? (
                            <span className="loading loading-spinner loading-xs" />
                          ) : (
                            <FolderOpen size={15} />
                          )}
                          {t("common.open", "Open")}
                        </button>
                      ) : null}
                      {candidate.allowed_resolutions?.includes("update_location") ? (
                        <button
                          type="button"
                          className="btn btn-secondary btn-sm"
                          onClick={() => void resolve(candidate, "update_location")}
                          disabled={
                            busy ||
                            ((candidate.risk_warnings?.length ?? 0) > 0 &&
                              !riskConfirmations[candidate.directory_name ?? ""])
                          }
                        >
                          <FolderOpen size={15} />
                          {t(
                            "manage.repositories.hostAction.updateLocation",
                            "Use as moved original",
                          )}
                        </button>
                      ) : null}
                      {candidate.allowed_resolutions?.includes("add_separate") ? (
                        <button
                          type="button"
                          className="btn btn-primary btn-sm"
                          onClick={() => setSeparateArmedFor(candidate.directory_name ?? null)}
                          disabled={
                            busy ||
                            ((candidate.risk_warnings?.length ?? 0) > 0 &&
                              !riskConfirmations[candidate.directory_name ?? ""])
                          }
                        >
                          <Copy size={15} />
                          {t(
                            "manage.repositories.hostAction.addSeparate",
                            "Add as separate Repository",
                          )}
                        </button>
                      ) : null}
                    </div>
                  </div>
                  {armed ? (
                    <div
                      role="alert"
                      className="alert alert-warning alert-soft mt-3 flex-wrap items-center gap-2 py-2 text-xs"
                    >
                      <AlertTriangle className="size-4 shrink-0" />
                      <span className="min-w-0 flex-1">
                        {t(
                          "manage.repositories.candidates.copyConfirmation",
                          "Add this as a separate Repository? Lumilio will create a new identity and isolate copied private state. This is not synchronization or a backup link.",
                        )}
                      </span>
                      <span className="flex shrink-0 items-center gap-2">
                        <button
                          type="button"
                          className="btn btn-ghost btn-xs"
                          onClick={() => setSeparateArmedFor(null)}
                          disabled={busy}
                        >
                          {t("common.cancel", { defaultValue: "Cancel" })}
                        </button>
                        <button
                          type="button"
                          className="btn btn-primary btn-xs"
                          onClick={() => void resolve(candidate, "add_separate")}
                          disabled={busy}
                        >
                          {t(
                            "manage.repositories.candidates.addSeparateConfirm",
                            "Yes, add as separate Repository",
                          )}
                        </button>
                      </span>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={onClose}
        disabled={busy}
        aria-label={t("common.close", { defaultValue: "Close" })}
      />
    </div>
  );
}

function candidateLabel(
  classification: string | undefined,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (classification) {
    case "registered_repository":
      return t("manage.repositories.candidates.registered", "Registered");
    case "existing_repository":
      return t("manage.repositories.candidates.openable", "Ready to open");
    case "empty_writable":
      return t("manage.repositories.candidates.emptyWritable", "Empty and writable");
    case "nonempty_unmarked":
      return t("manage.repositories.candidates.nonempty", "Not a Repository");
    case "marker_invalid":
      return t("manage.repositories.candidates.invalidMarker", "Invalid Repository marker");
    case "identity_error":
      return t("manage.repositories.candidates.identityError", "Identity already registered");
    default:
      return t("manage.repositories.candidates.unavailableStatus", "Unavailable");
  }
}

function candidateBadgeClass(classification: string | undefined): string {
  if (classification === "existing_repository" || classification === "empty_writable") {
    return "badge-success badge-soft";
  }
  if (classification === "registered_repository") return "badge-info badge-soft";
  if (classification === "nonempty_unmarked") return "badge-warning badge-soft";
  return "badge-error badge-soft";
}
