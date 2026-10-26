import { useEffect, useState } from "react";
import { AlertTriangle, Copy, FolderOpen, RefreshCw, X } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import {
  localizeAPIProblem,
  localizeProblemType,
  normalizeProblem,
} from "@/lib/http-commons/problem";
import {
  type RepositoryCandidate,
  useOpenRepositoryCandidate,
  useRepositoryCandidates,
  useResolveRepositoryCandidate,
} from "../../api/useRepositoryCandidates";
import { useStorageAssessment } from "../../api/useStorageAssessment";
import { StorageRiskConfirmation } from "../../components/StorageRiskConfirmation";
import { ProblemRecoveryLink } from "@/components/problem/ProblemRecoveryLink";

type CandidateAction = "open" | "update_location" | "add_separate";

/** The Server asks for a fresh review of the same folder on these answers. */
const CONSENT_PROBLEMS = new Set([
  "https://lumilio.org/problems/storage/confirmation-required",
  "https://lumilio.org/problems/storage/assessment-stale",
]);

/**
 * Docker and standalone candidates: direct children of the Default Storage
 * Location the Server classified. A candidate only suggests a folder; the
 * decision to open it, use it as the moved original, or add it as a separate
 * Repository goes through the same Server review and the same risk
 * acknowledgement as a create, bound to that exact folder by the review's
 * token. Nothing here classifies the folder itself.
 */
export default function RepositoryCandidateModal({
  isOpen,
  onClose,
  focusDirectoryName,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The folder a create handed over, highlighted so the journey continues where it stopped. */
  focusDirectoryName?: string;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const candidatesQuery = useRepositoryCandidates(isOpen);
  const openMutation = useOpenRepositoryCandidate();
  const resolveMutation = useResolveRepositoryCandidate();
  const candidates = candidatesQuery.data?.candidates ?? [];
  const busy = openMutation.isPending || resolveMutation.isPending;
  const [pending, setPending] = useState<{ directoryName: string; action: CandidateAction } | null>(
    null,
  );
  const [acknowledged, setAcknowledged] = useState(false);
  const [reviewNotice, setReviewNotice] = useState<string | null>(null);

  const reviewQuery = useStorageAssessment(
    { intent: "open_repository", directoryName: pending?.directoryName },
    isOpen && pending !== null,
  );
  const review = pending ? reviewQuery.data : undefined;
  const reviewRisks = review?.risks ?? [];
  const reviewProblem = review?.problem
    ? localizeProblemType(
        `https://lumilio.org/problems/${review.problem}`,
        t,
        t(
          "manage.repositories.review.unavailable",
          "This target cannot be used for a Repository right now.",
        ),
      )
    : null;
  const reviewToken = review?.token;
  // An acknowledgement belongs to one token. New facts, a new token, a new review.
  useEffect(() => {
    setAcknowledged(false);
  }, [reviewToken]);

  const beginAction = (candidate: RepositoryCandidate, action: CandidateAction) => {
    if (!candidate.directory_name) return;
    setReviewNotice(null);
    setAcknowledged(false);
    setPending({ directoryName: candidate.directory_name, action });
  };

  const cancelAction = () => {
    setPending(null);
    setReviewNotice(null);
    setAcknowledged(false);
  };

  const confirmAction = async () => {
    if (!pending || !review || review.problem) return;
    if (reviewRisks.length > 0 && !acknowledged) return;
    const consent = { assessmentToken: review.token, acknowledgedRisks: reviewRisks };
    try {
      if (pending.action === "open") {
        await openMutation.openRepositoryCandidate(pending.directoryName, consent);
        showMessage(
          "success",
          t(
            "manage.repositories.candidates.opened",
            "Repository opened and its initial scan was queued.",
          ),
        );
      } else {
        await resolveMutation.resolveRepositoryCandidate(
          pending.directoryName,
          pending.action,
          consent,
        );
        showMessage(
          "success",
          pending.action === "update_location"
            ? t("manage.repositories.candidates.relocated", "Repository location updated.")
            : t("manage.repositories.candidates.copyAdded", "Separate Repository added."),
        );
      }
      cancelAction();
      onClose();
    } catch (reason: unknown) {
      const problem = normalizeProblem(reason);
      const fallback =
        pending.action === "open"
          ? t("manage.repositories.candidates.openFailed", "Repository could not be opened.")
          : t(
              "manage.repositories.candidates.resolveFailed",
              "Repository decision could not be applied.",
            );
      if (problem.kind === "problem" && CONSENT_PROBLEMS.has(problem.type)) {
        // The folder's facts changed between review and command, or the
        // acknowledgement did not cover them: review the current facts again.
        setAcknowledged(false);
        setReviewNotice(localizeAPIProblem(reason, t, fallback));
        void reviewQuery.refetch();
        return;
      }
      showMessage("error", localizeAPIProblem(reason, t, fallback));
    }
  };

  if (!isOpen) return null;

  const confirmLabel = (action: CandidateAction) => {
    switch (action) {
      case "open":
        return t("common.open", "Open");
      case "update_location":
        return t("manage.repositories.hostAction.updateLocation", "Use as moved original");
      default:
        return t(
          "manage.repositories.candidates.addSeparateConfirm",
          "Yes, add as separate Repository",
        );
    }
  };

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
              const isPending = pending?.directoryName === candidate.directory_name;
              return (
                <div
                  key={candidate.directory_name}
                  data-testid="repository-candidate"
                  data-directory={candidate.directory_name}
                  className={`rounded-lg border px-4 py-3 ${focusDirectoryName && candidate.directory_name === focusDirectoryName ? "border-primary ring-1 ring-primary/40" : "border-base-300"}`}
                  data-focused={
                    focusDirectoryName && candidate.directory_name === focusDirectoryName
                      ? "true"
                      : undefined
                  }
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
                    </div>
                    {isPending ? null : (
                      <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
                        {candidate.can_open ? (
                          <button
                            type="button"
                            className="btn btn-primary btn-sm"
                            onClick={() => beginAction(candidate, "open")}
                            disabled={busy || pending !== null}
                          >
                            <FolderOpen size={15} />
                            {t("common.open", "Open")}
                          </button>
                        ) : null}
                        {candidate.allowed_resolutions?.includes("update_location") ? (
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            onClick={() => beginAction(candidate, "update_location")}
                            disabled={busy || pending !== null}
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
                            onClick={() => beginAction(candidate, "add_separate")}
                            disabled={busy || pending !== null}
                          >
                            <Copy size={15} />
                            {t(
                              "manage.repositories.hostAction.addSeparate",
                              "Add as separate Repository",
                            )}
                          </button>
                        ) : null}
                      </div>
                    )}
                  </div>
                  {isPending && pending ? (
                    <section
                      aria-label={t("manage.repositories.candidates.reviewTitle", "Folder review")}
                      className="mt-3 space-y-3"
                    >
                      {pending.action === "add_separate" ? (
                        <div
                          role="note"
                          className="alert alert-warning alert-soft flex-wrap items-center gap-2 py-2 text-xs"
                        >
                          <AlertTriangle className="size-4 shrink-0" />
                          <span className="min-w-0 flex-1">
                            {t(
                              "manage.repositories.candidates.copyConfirmation",
                              "Add this as a separate Repository? Lumilio will create a new identity and isolate copied private state. This is not synchronization or a backup link.",
                            )}
                          </span>
                        </div>
                      ) : null}
                      {reviewQuery.isPending ? (
                        <div className="flex items-center gap-2 text-xs text-base-content/60">
                          <span className="loading loading-spinner loading-xs" />
                          {t(
                            "manage.repositories.review.assessing",
                            "Reviewing the storage target…",
                          )}
                        </div>
                      ) : null}
                      {reviewQuery.isError ? (
                        <div role="alert" className="alert alert-error alert-soft py-2 text-sm">
                          <span>
                            {localizeAPIProblem(
                              reviewQuery.error,
                              t,
                              t(
                                "manage.repositories.review.failed",
                                "The target could not be reviewed.",
                              ),
                            )}
                          </span>
                          <button
                            type="button"
                            className="btn btn-sm"
                            onClick={() => void reviewQuery.refetch()}
                          >
                            <RefreshCw size={15} /> {t("common.retry")}
                          </button>
                        </div>
                      ) : null}
                      {reviewProblem && review?.problem ? (
                        <div role="alert" className="alert alert-error alert-soft py-2 text-sm">
                          <span>{reviewProblem}</span>
                          <ProblemRecoveryLink
                            problem={{ type: `https://lumilio.org/problems/${review.problem}` }}
                          />
                        </div>
                      ) : null}
                      {reviewNotice ? (
                        <div role="status" className="alert alert-warning alert-soft py-2 text-sm">
                          <RefreshCw size={16} className="shrink-0" />
                          <span>{reviewNotice}</span>
                        </div>
                      ) : null}
                      {review && !review.problem ? (
                        <StorageRiskConfirmation
                          risks={reviewRisks}
                          checked={acknowledged}
                          onChange={setAcknowledged}
                          disabled={busy}
                        />
                      ) : null}
                      <div className="flex flex-wrap items-center justify-end gap-2">
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={cancelAction}
                          disabled={busy}
                        >
                          {t("common.cancel", { defaultValue: "Cancel" })}
                        </button>
                        <button
                          type="button"
                          className="btn btn-primary btn-sm"
                          onClick={() => void confirmAction()}
                          disabled={
                            busy ||
                            !review ||
                            Boolean(review.problem) ||
                            (reviewRisks.length > 0 && !acknowledged)
                          }
                        >
                          {busy ? (
                            <span className="loading loading-spinner loading-xs" />
                          ) : pending.action === "add_separate" ? (
                            <Copy size={15} />
                          ) : (
                            <FolderOpen size={15} />
                          )}
                          {confirmLabel(pending.action)}
                        </button>
                      </div>
                    </section>
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
    case "unavailable":
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
