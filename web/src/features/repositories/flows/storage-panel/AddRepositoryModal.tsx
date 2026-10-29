import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import {
  AlertTriangle,
  Check,
  ChevronLeft,
  ChevronRight,
  FolderPlus,
  HardDrive,
  LayoutGrid,
  RefreshCw,
  X,
} from "lucide-react";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n.tsx";
import {
  localizeAPIProblem,
  localizeProblemType,
  normalizeProblem,
} from "@/lib/http-commons/problem";
import { useCreateRepository } from "../../api/useCreateRepository";
import { useRepositoryCandidates } from "../../api/useRepositoryCandidates";
import { useStorageAssessment } from "../../api/useStorageAssessment";
import { useStorageLocations } from "../../api/useStorageLocations";
import {
  StorageStrategyPicker,
  type RepositoryStorageStrategy,
} from "../../components/StorageStrategyPicker";
import { StorageRiskConfirmation } from "../../components/StorageRiskConfirmation";
import { ProblemRecoveryLink } from "@/components/problem/ProblemRecoveryLink";
import {
  validateRepositoryDirectoryName,
  validateRepositoryName,
  type RepositoryDirectoryNameError,
  type RepositoryNameError,
} from "../../model/repositorySetup";
import { getStorageEntityDisplayName } from "../../model/storageEntities";

type CreateRepositoryStep = 0 | 1 | 2 | 3;

/**
 * A create that ended in a Repository conflict the user must resolve through
 * another journey (Open Existing, reconnect, diagnose). The Storage Location
 * and folder are retained so that journey starts where the user already was.
 */
export type CreateRepositoryRecovery = {
  conflictType: string;
  storageLocationId: string;
  directoryName: string;
  repositoryId?: string;
};

const RECOVERABLE_CONFLICTS = new Set([
  "existing_repository_found",
  "repository_identity",
  "repository_marker_invalid",
]);

const CONSENT_PROBLEMS = new Set([
  "https://lumilio.org/problems/storage/confirmation-required",
  "https://lumilio.org/problems/storage/assessment-stale",
]);

export default function AddRepositoryModal({
  isOpen,
  onClose,
  canRequestStorageLocation = false,
  onRequestStorageLocation,
  showServerCandidates = false,
  onRecoveryRequired,
  initialStorageLocationId,
}: {
  isOpen: boolean;
  onClose: () => void;
  canRequestStorageLocation?: boolean;
  onRequestStorageLocation?: () => void;
  showServerCandidates?: boolean;
  onRecoveryRequired?: (recovery: CreateRepositoryRecovery) => void;
  initialStorageLocationId?: string;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const createRepositoryMutation = useCreateRepository();
  const storageLocationsQuery = useStorageLocations();
  const candidatesQuery = useRepositoryCandidates(isOpen && showServerCandidates);
  const [name, setName] = useState("");
  const [directoryName, setDirectoryName] = useState("");
  const [directoryNameCustomized, setDirectoryNameCustomized] = useState(false);
  const [storageLocationId, setStorageLocationId] = useState("");
  const [storageStrategy, setStorageStrategy] = useState<RepositoryStorageStrategy>("date");
  const [riskAcknowledged, setRiskAcknowledged] = useState(false);
  const [reviewNotice, setReviewNotice] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitProblem, setSubmitProblem] = useState<unknown>(null);
  const [step, setStep] = useState<CreateRepositoryStep>(0);
  const nameError = validateRepositoryName(name);
  const nameErrorMessage = repositoryNameErrorMessage(nameError, t);
  const directoryNameError = validateRepositoryDirectoryName(directoryName);
  const directoryNameErrorMessage = repositoryDirectoryNameErrorMessage(directoryNameError, t);
  const basicStepValid = nameError === null && directoryNameError === null;

  const storageLocations = storageLocationsQuery.data?.storage_locations ?? [];
  const activeStorageLocations = useMemo(
    () => storageLocations.filter((location) => Boolean(location.id)),
    [storageLocations],
  );
  const selectedStorageLocation = useMemo(
    () => storageLocations.find((location) => location.id === storageLocationId),
    [storageLocationId, storageLocations],
  );
  const creatableCandidates = useMemo(
    () => (candidatesQuery.data?.candidates ?? []).filter((candidate) => candidate.can_create),
    [candidatesQuery.data?.candidates],
  );
  // The Server reviews the actual target (this Storage Location, this folder)
  // when the Review step opens. Candidates only suggest folder names; they
  // never supply the risks of a different location.
  const reviewEnabled = isOpen && step === 3 && Boolean(storageLocationId) && basicStepValid;
  const assessmentQuery = useStorageAssessment(
    { intent: "create_repository", storageLocationId, directoryName },
    reviewEnabled,
  );
  const assessment = reviewEnabled ? assessmentQuery.data : undefined;
  const reviewRisks = useMemo(() => assessment?.risks ?? [], [assessment?.risks]);
  const reviewProblem = assessment?.problem
    ? localizeProblemType(
        `https://lumilio.org/problems/${assessment.problem}`,
        t,
        t(
          "manage.repositories.review.unavailable",
          "This target cannot be used for a Repository right now.",
        ),
      )
    : null;
  const assessmentToken = assessment?.token;
  // An acknowledgement belongs to one token. New facts, a new token, a new review.
  useEffect(() => {
    setRiskAcknowledged(false);
  }, [assessmentToken]);

  useEffect(() => {
    if (!isOpen || activeStorageLocations.length === 0) return;
    setStorageLocationId((current) => {
      if (
        initialStorageLocationId &&
        activeStorageLocations.some((root) => root.id === initialStorageLocationId)
      ) {
        return initialStorageLocationId;
      }
      if (activeStorageLocations.some((root) => root.id === current)) return current;
      return (
        activeStorageLocations.find((root) => root.kind === "default")?.id ??
        activeStorageLocations[0]?.id ??
        ""
      );
    });
  }, [activeStorageLocations, initialStorageLocationId, isOpen]);

  const resetForm = useCallback(() => {
    setName("");
    setDirectoryName("");
    setDirectoryNameCustomized(false);
    setStorageLocationId("");
    setStorageStrategy("date");
    setRiskAcknowledged(false);
    setReviewNotice(null);
    setSubmitError(null);
    setStep(0);
  }, []);

  const handleClose = useCallback(() => {
    if (createRepositoryMutation.isPending) return;
    resetForm();
    onClose();
  }, [createRepositoryMutation.isPending, onClose, resetForm]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (step < 3) {
        if (
          (step === 0 &&
            (validateRepositoryName(name) || validateRepositoryDirectoryName(directoryName))) ||
          (step === 1 && !storageLocationId) ||
          createRepositoryMutation.isPending
        ) {
          return;
        }
        setStep((step + 1) as CreateRepositoryStep);
        return;
      }
      if (
        validateRepositoryName(name) ||
        validateRepositoryDirectoryName(directoryName) ||
        createRepositoryMutation.isPending
      )
        return;
      if (!storageLocationId || !assessment || assessment.problem) return;
      if (reviewRisks.length > 0 && !riskAcknowledged) return;

      setSubmitError(null);
      setSubmitProblem(null);
      setReviewNotice(null);
      try {
        await createRepositoryMutation.createRepository({
          name,
          directoryName,
          storageLocationId,
          storageStrategy,
          assessmentToken: assessment.token,
          acknowledgedRisks: reviewRisks,
        });
        showMessage("success", t("manage.repositories.createSuccess", { name }));
        resetForm();
        onClose();
      } catch (error) {
        const problem = normalizeProblem(error);
        if (problem.kind === "problem" && CONSENT_PROBLEMS.has(problem.type)) {
          // The target's facts changed between review and command, or the
          // acknowledgement did not cover them: review the current facts again.
          setRiskAcknowledged(false);
          setReviewNotice(localizeAPIProblem(error, t, t("manage.repositories.createFailed")));
          void assessmentQuery.refetch();
          return;
        }
        if (
          problem.kind === "problem" &&
          problem.conflictType &&
          RECOVERABLE_CONFLICTS.has(problem.conflictType) &&
          onRecoveryRequired
        ) {
          const recovery: CreateRepositoryRecovery = {
            conflictType: problem.conflictType,
            storageLocationId,
            directoryName,
            repositoryId: problem.repositoryID,
          };
          resetForm();
          onClose();
          onRecoveryRequired(recovery);
          return;
        }
        setSubmitError(localizeAPIProblem(error, t, t("manage.repositories.createFailed")));
        setSubmitProblem(error);
      }
    },
    [
      assessment,
      assessmentQuery,
      createRepositoryMutation,
      directoryName,
      name,
      onClose,
      onRecoveryRequired,
      resetForm,
      reviewRisks,
      riskAcknowledged,
      showMessage,
      storageLocationId,
      storageStrategy,
      step,
      t,
    ],
  );

  if (!isOpen) return null;

  const wizardSteps = [
    t("manage.repositories.createWizard.details", "Details"),
    t("manage.repositories.createWizard.location", "Storage Location"),
    t("manage.repositories.createWizard.layout", "Storage layout"),
    t("manage.repositories.createWizard.review", "Review"),
  ];

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-2xl p-0">
        <div className="flex items-start justify-between gap-4 border-b border-base-300 px-6 py-5">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <FolderPlus size={20} />
            </div>
            <div>
              <h3 className="text-base font-semibold">{t("manage.repositories.createTitle")}</h3>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={handleClose}
            disabled={createRepositoryMutation.isPending}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        <ol
          className="grid grid-cols-4 border-b border-base-300 px-6"
          aria-label={t("manage.repositories.createWizard.progress", "Creation progress")}
        >
          {wizardSteps.map((label, index) => (
            <li
              key={label}
              className={`relative flex min-w-0 items-center gap-2 border-b-2 py-3 text-xs font-medium ${index === step ? "border-primary text-primary" : index < step ? "border-transparent text-base-content" : "border-transparent text-base-content/45"}`}
              aria-current={index === step ? "step" : undefined}
            >
              <span
                className={`flex size-5 shrink-0 items-center justify-center rounded-full text-[11px] ${index < step ? "bg-primary text-primary-content" : index === step ? "bg-primary/15 text-primary" : "bg-base-200 text-base-content/55"}`}
              >
                {index < step ? <Check size={12} /> : index + 1}
              </span>
              <span className="hidden truncate sm:block">{label}</span>
            </li>
          ))}
        </ol>

        <form onSubmit={handleSubmit}>
          <div className="min-h-[22rem] px-6 py-5">
            {step === 0 ? (
              <section aria-labelledby="create-repository-details-step" className="space-y-5">
                <div>
                  <h4 id="create-repository-details-step" className="text-sm font-semibold">
                    {t("manage.repositories.createWizard.detailsTitle", "Name the Repository")}
                  </h4>
                  <p className="mt-1 max-w-xl text-sm text-base-content/60">
                    {t(
                      "manage.repositories.createWizard.detailsDescription",
                      "Choose the name shown in Lumilio. The storage folder follows it unless you customize the folder below.",
                    )}
                  </p>
                </div>
                <div className="fieldset w-full gap-1">
                  <label
                    className="fieldset-legend p-0 text-sm font-medium"
                    htmlFor="repository-name"
                  >
                    {t("manage.repositories.createNameLabel")}
                  </label>
                  <input
                    id="repository-name"
                    type="text"
                    className={`input input-bordered w-full ${name.length > 0 && nameError ? "input-error" : ""}`}
                    value={name}
                    onChange={(event) => {
                      const nextName = event.target.value;
                      setName(nextName);
                      if (!directoryNameCustomized) setDirectoryName(nextName);
                    }}
                    placeholder={t("manage.repositories.createNamePlaceholder")}
                    disabled={createRepositoryMutation.isPending}
                    aria-invalid={name.length > 0 && nameError !== null}
                    aria-describedby="repository-name-hint"
                    autoFocus
                    required
                  />
                  <span
                    id="repository-name-hint"
                    className={`label text-xs leading-snug ${name.length > 0 && nameError ? "text-error" : "text-base-content/55"}`}
                  >
                    {name.length > 0 && nameError
                      ? nameErrorMessage
                      : t(
                          "manage.repositories.createNameHint",
                          "This display name can be changed later without moving files.",
                        )}
                  </span>
                </div>

                <details
                  className="rounded-lg border border-base-300 bg-base-100 px-4 py-3"
                  open={directoryNameCustomized || directoryNameError !== null}
                >
                  <summary className="cursor-pointer text-sm font-medium">
                    {t(
                      "manage.repositories.createWizard.advancedFolder",
                      "Advanced: storage folder",
                    )}
                  </summary>
                  <div className="fieldset mt-3 w-full gap-1">
                    <label
                      className="fieldset-legend p-0 text-sm font-medium"
                      htmlFor="repository-directory-name"
                    >
                      {t("manage.repositories.createDirectoryLabel", "Storage folder")}
                    </label>
                    <input
                      id="repository-directory-name"
                      type="text"
                      className={`input input-bordered w-full ${directoryName.length > 0 && directoryNameError ? "input-error" : ""}`}
                      value={directoryName}
                      onChange={(event) => {
                        setDirectoryName(event.target.value);
                        setDirectoryNameCustomized(true);
                      }}
                      placeholder={t(
                        "manage.repositories.createDirectoryPlaceholder",
                        "family-media",
                      )}
                      disabled={createRepositoryMutation.isPending}
                      aria-invalid={directoryName.length > 0 && directoryNameError !== null}
                      aria-describedby="repository-directory-name-hint"
                      required
                    />
                    <span
                      id="repository-directory-name-hint"
                      className={`label text-xs leading-snug ${directoryName.length > 0 && directoryNameError ? "text-error" : "text-base-content/55"}`}
                    >
                      {directoryName.length > 0 && directoryNameError
                        ? directoryNameErrorMessage
                        : t(
                            "manage.repositories.createDirectoryHint",
                            "A stable folder directly inside the selected Storage Location.",
                          )}
                    </span>
                    {showServerCandidates && creatableCandidates.length > 0 ? (
                      <div
                        className="mt-2 flex flex-wrap gap-2"
                        aria-label={t(
                          "manage.repositories.candidates.emptyWritable",
                          "Empty and writable",
                        )}
                      >
                        {creatableCandidates.map((candidate) => (
                          <button
                            key={candidate.directory_name}
                            type="button"
                            className={`btn btn-xs ${directoryName === candidate.directory_name ? "btn-primary" : "btn-soft"}`}
                            onClick={() => {
                              setDirectoryName(candidate.directory_name ?? "");
                              setDirectoryNameCustomized(true);
                            }}
                          >
                            {candidate.directory_name}
                            {candidate.mount_point
                              ? ` · ${t("manage.repositories.candidates.mountPoint", "mounted")}`
                              : ""}
                          </button>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </details>
              </section>
            ) : null}

            {step === 1 ? (
              <section aria-labelledby="create-repository-location-step" className="space-y-5">
                <div>
                  <h4 id="create-repository-location-step" className="text-sm font-semibold">
                    {t(
                      "manage.repositories.createWizard.locationTitle",
                      "Choose a Storage Location",
                    )}
                  </h4>
                  <p className="mt-1 max-w-xl text-sm text-base-content/60">
                    {t(
                      "manage.repositories.createWizard.locationDescription",
                      "A Storage Location is an authorized parent location. Lumilio creates this Repository as its direct child.",
                    )}
                  </p>
                </div>
                <div className="fieldset gap-1">
                  <label
                    className="fieldset-legend p-0 text-sm font-medium"
                    htmlFor="repository-storage-location"
                  >
                    {t("manage.repositories.storageLocationLabel", "Storage Location")}
                  </label>
                  <select
                    id="repository-storage-location"
                    className="select select-bordered w-full"
                    value={storageLocationId}
                    onChange={(event) => {
                      setStorageLocationId(event.target.value);
                      setRiskAcknowledged(false);
                    }}
                    disabled={createRepositoryMutation.isPending || storageLocationsQuery.isLoading}
                    required
                  >
                    {activeStorageLocations.length === 0 ? (
                      <option value="">
                        {storageLocationsQuery.isLoading
                          ? t("manage.repositories.storageLocationLoading", "Loading locations…")
                          : t("manage.repositories.storageLocationEmpty", "No writable location")}
                      </option>
                    ) : null}
                    {activeStorageLocations.map((location) => (
                      <option key={location.id} value={location.id}>
                        {getStorageEntityDisplayName(location, t)}
                      </option>
                    ))}
                  </select>
                </div>
                {selectedStorageLocation ? (
                  <div className="rounded-xl border border-base-300 bg-base-200/35 p-4">
                    <div className="flex items-start gap-3">
                      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-base-200 text-base-content/70">
                        <HardDrive size={18} />
                      </span>
                      <div className="min-w-0">
                        <div className="font-medium">
                          {getStorageEntityDisplayName(selectedStorageLocation, t)}
                        </div>
                        <div className="mt-2 text-xs text-base-content/65">
                          {t(
                            "manage.repositories.storageLocationRepositoryCount",
                            "{{count}} Repositories",
                            {
                              count: selectedStorageLocation.repository_count ?? 0,
                            },
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                ) : null}
                <p className="text-xs leading-snug text-base-content/55">
                  {t(
                    "manage.repositories.storageLocationHint",
                    "A Storage Location can contain multiple Repositories. Authorize external Storage Locations in the Desktop Control Panel.",
                  )}
                </p>
                {!storageLocationsQuery.isLoading &&
                activeStorageLocations.length === 0 &&
                canRequestStorageLocation ? (
                  <button
                    type="button"
                    className="btn btn-soft btn-sm self-start"
                    onClick={onRequestStorageLocation}
                  >
                    <FolderPlus size={15} />
                    {t(
                      "manage.repositories.hostAction.requestLocation",
                      "Request a Storage Location from Desktop",
                    )}
                  </button>
                ) : null}
                {storageLocationsQuery.isError ? (
                  <div role="alert" className="alert alert-error alert-soft py-2 text-xs">
                    {t(
                      "manage.repositories.storageLocationError",
                      "Storage Locations are unavailable.",
                    )}
                  </div>
                ) : null}
              </section>
            ) : null}

            {step === 2 ? (
              <section aria-labelledby="create-repository-layout-step" className="space-y-5">
                <div>
                  <h4 id="create-repository-layout-step" className="text-sm font-semibold">
                    {t(
                      "manage.repositories.createWizard.layoutTitle",
                      "Choose how new files are placed",
                    )}
                  </h4>
                  <p className="mt-1 max-w-xl text-sm text-base-content/60">
                    {t(
                      "manage.repositories.createWizard.layoutDescription",
                      "This controls the on-disk layout for future imports and cannot be changed after creation.",
                    )}
                  </p>
                </div>
                <StorageStrategyPicker
                  value={storageStrategy}
                  onChange={setStorageStrategy}
                  disabled={createRepositoryMutation.isPending}
                  idPrefix="manage-repository"
                />
              </section>
            ) : null}

            {step === 3 ? (
              <section aria-labelledby="create-repository-review-step" className="space-y-5">
                <div>
                  <h4 id="create-repository-review-step" className="text-sm font-semibold">
                    {t("manage.repositories.createWizard.reviewTitle", "Review and create")}
                  </h4>
                  <p className="mt-1 max-w-xl text-sm text-base-content/60">
                    {t(
                      "manage.repositories.createWizard.reviewDescription",
                      "Confirm the Repository identity and its permanent storage layout before creating it.",
                    )}
                  </p>
                </div>
                <dl className="divide-y divide-base-300 rounded-xl border border-base-300 bg-base-100">
                  <ReviewRow label={t("manage.repositories.createNameLabel")} value={name} />
                  <ReviewRow
                    label={t("manage.repositories.createDirectoryLabel", "Storage folder")}
                    value={directoryName}
                    mono
                  />
                  <ReviewRow
                    label={t("manage.repositories.storageLocationLabel", "Storage Location")}
                    value={
                      selectedStorageLocation
                        ? getStorageEntityDisplayName(selectedStorageLocation, t)
                        : "—"
                    }
                  />
                  <ReviewRow
                    label={t("manage.repositories.storageStrategy.label", "Storage layout")}
                    value={storageStrategyLabel(storageStrategy, t)}
                    icon={<LayoutGrid size={15} />}
                  />
                </dl>
                {assessmentQuery.isPending && reviewEnabled ? (
                  <p className="flex items-center gap-2 text-sm text-base-content/60" role="status">
                    <span className="loading loading-spinner loading-xs" />
                    {t("manage.repositories.review.assessing", "Reviewing the storage target…")}
                  </p>
                ) : null}
                {assessmentQuery.isError ? (
                  <div role="alert" className="alert alert-error alert-soft py-2 text-sm">
                    <span className="min-w-0 flex-1">
                      {localizeAPIProblem(
                        assessmentQuery.error,
                        t,
                        t(
                          "manage.repositories.review.failed",
                          "The storage target could not be reviewed.",
                        ),
                      )}
                    </span>
                    <button
                      type="button"
                      className="btn btn-ghost btn-xs gap-1"
                      onClick={() => void assessmentQuery.refetch()}
                    >
                      <RefreshCw size={14} />
                      {t("manage.repositories.review.retry", "Review again")}
                    </button>
                  </div>
                ) : null}
                {reviewProblem ? (
                  <div role="alert" className="alert alert-warning alert-soft py-2 text-sm">
                    <AlertTriangle size={16} className="shrink-0" />
                    <span>{reviewProblem}</span>
                  </div>
                ) : null}
                {reviewNotice ? (
                  <div role="status" className="alert alert-info alert-soft py-2 text-sm">
                    <RefreshCw size={16} className="shrink-0" />
                    <span>{reviewNotice}</span>
                  </div>
                ) : null}
                {assessment && !assessment.problem ? (
                  <StorageRiskConfirmation
                    risks={reviewRisks}
                    checked={riskAcknowledged}
                    onChange={setRiskAcknowledged}
                    disabled={createRepositoryMutation.isPending}
                  />
                ) : null}
                {submitError ? (
                  <div role="alert" className="alert alert-error alert-soft py-2 text-sm">
                    <span>{submitError}</span>
                    <ProblemRecoveryLink problem={submitProblem} />
                  </div>
                ) : null}
              </section>
            ) : null}
          </div>

          <div className="flex items-center justify-between border-t border-base-300 px-6 py-4">
            <div className="text-xs text-base-content/50">
              {t("manage.repositories.createWizard.stepCount", "Step {{current}} of {{total}}", {
                current: step + 1,
                total: wizardSteps.length,
              })}
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() =>
                  step === 0 ? handleClose() : setStep((step - 1) as CreateRepositoryStep)
                }
                disabled={createRepositoryMutation.isPending}
              >
                {step === 0 ? (
                  t("common.cancel", { defaultValue: "Cancel" })
                ) : (
                  <>
                    <ChevronLeft size={16} />
                    {t("common.back", "Back")}
                  </>
                )}
              </button>
              {step < 3 ? (
                <button
                  type="button"
                  className="btn btn-primary gap-2"
                  disabled={
                    (step === 0 && !basicStepValid) ||
                    (step === 1 && !storageLocationId) ||
                    createRepositoryMutation.isPending
                  }
                  onClick={(event) => {
                    event.preventDefault();
                    setStep((step + 1) as CreateRepositoryStep);
                  }}
                >
                  {t("common.next", "Next")}
                  <ChevronRight size={16} />
                </button>
              ) : (
                <button
                  type="submit"
                  className="btn btn-primary gap-2"
                  disabled={
                    createRepositoryMutation.isPending ||
                    !assessment ||
                    Boolean(assessment.problem) ||
                    (reviewRisks.length > 0 && !riskAcknowledged)
                  }
                >
                  {createRepositoryMutation.isPending ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <FolderPlus size={16} />
                  )}
                  {t("manage.repositories.createSubmit")}
                </button>
              )}
            </div>
          </div>
        </form>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={handleClose}
        aria-label={t("common.close", { defaultValue: "Close" })}
      />
    </div>
  );
}

function ReviewRow({
  label,
  value,
  detail,
  mono = false,
  icon,
}: {
  label: string;
  value: string;
  detail?: string;
  mono?: boolean;
  icon?: ReactNode;
}) {
  return (
    <div className="grid gap-1 px-4 py-3 sm:grid-cols-[9rem_1fr] sm:gap-4">
      <dt className="text-xs font-medium text-base-content/55">{label}</dt>
      <dd className="min-w-0 text-sm">
        <span className={`flex items-center gap-2 ${mono ? "font-mono text-xs" : "font-medium"}`}>
          {icon}
          {value}
        </span>
        {detail ? (
          <span
            className="mt-0.5 block truncate font-mono text-[11px] text-base-content/50"
            title={detail}
          >
            {detail}
          </span>
        ) : null}
      </dd>
    </div>
  );
}

function storageStrategyLabel(
  strategy: RepositoryStorageStrategy,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (strategy) {
    case "flat":
      return t("manage.repositories.storageStrategy.flatLabel", "Single folder");
    case "cas":
      return t("manage.repositories.storageStrategy.casLabel", "Content addressed");
    default:
      return t("manage.repositories.storageStrategy.dateLabel", "By date");
  }
}

function repositoryNameErrorMessage(
  error: RepositoryNameError | null,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (error) {
    case "required":
      return t("manage.repositories.createNameRequired", "Enter a Repository name.");
    case "leadingOrTrailingSpace":
      return t(
        "manage.repositories.createNameEdgeSpace",
        "The name cannot start or end with a space.",
      );
    case "tooManyCharacters":
      return t(
        "manage.repositories.createNameTooManyCharacters",
        "The name cannot exceed 80 characters.",
      );
    case "tooManyBytes":
      return t(
        "manage.repositories.createNameTooManyBytes",
        "The name cannot exceed 240 UTF-8 bytes.",
      );
    case "controlCharacter":
      return t(
        "manage.repositories.createNameControlCharacter",
        "The name cannot contain control characters.",
      );
    default:
      return "";
  }
}

function repositoryDirectoryNameErrorMessage(
  error: RepositoryDirectoryNameError | null,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (error) {
    case "required":
      return t("manage.repositories.createDirectoryRequired", "Enter a storage folder.");
    case "leadingOrTrailingSpace":
      return t(
        "manage.repositories.createDirectoryEdgeSpace",
        "The storage folder cannot start or end with a space.",
      );
    case "tooManyCharacters":
      return t(
        "manage.repositories.createDirectoryTooManyCharacters",
        "The storage folder cannot exceed 80 characters.",
      );
    case "tooManyBytes":
      return t(
        "manage.repositories.createDirectoryTooManyBytes",
        "The storage folder is too long when encoded for the filesystem.",
      );
    case "controlCharacter":
      return t(
        "manage.repositories.createDirectoryControlCharacter",
        "The storage folder cannot contain control characters.",
      );
    case "unsupportedCharacter":
      return t(
        "manage.repositories.createDirectoryUnsupportedCharacter",
        "Use only letters, numbers, spaces, hyphens, and underscores.",
      );
    default:
      return "";
  }
}
