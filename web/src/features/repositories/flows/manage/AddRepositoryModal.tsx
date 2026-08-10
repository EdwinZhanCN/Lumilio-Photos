import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import {
  Check,
  ChevronLeft,
  ChevronRight,
  FolderPlus,
  HardDrive,
  LayoutGrid,
  X,
} from "lucide-react";
import { useMessage } from "@/features/notifications";
import { formatBytes } from "@/lib/utils/formatters";
import { useI18n } from "@/lib/i18n.tsx";
import { useCreateRepository } from "../../api/useCreateRepository";
import { useRepositoryCandidates } from "../../api/useRepositoryCandidates";
import { useRepositoryRoots } from "../../api/useRepositoryRoots";
import {
  StorageStrategyPicker,
  type RepositoryStorageStrategy,
} from "../../components/StorageStrategyPicker";
import { StorageRiskConfirmation } from "../../components/StorageRiskConfirmation";
import {
  validateRepositoryDirectoryName,
  validateRepositoryName,
  type RepositoryDirectoryNameError,
  type RepositoryNameError,
} from "../../model/repositorySetup";

type CreateRepositoryStep = 0 | 1 | 2 | 3;

export default function AddRepositoryModal({
  isOpen,
  onClose,
  canRequestStorageLocation = false,
  onRequestStorageLocation,
  showServerCandidates = false,
  onRecoveryRequired,
}: {
  isOpen: boolean;
  onClose: () => void;
  canRequestStorageLocation?: boolean;
  onRequestStorageLocation?: () => void;
  showServerCandidates?: boolean;
  onRecoveryRequired?: (conflictType: string) => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const createRepositoryMutation = useCreateRepository();
  const rootsQuery = useRepositoryRoots();
  const candidatesQuery = useRepositoryCandidates(isOpen && showServerCandidates);
  const [name, setName] = useState("");
  const [directoryName, setDirectoryName] = useState("");
  const [directoryNameCustomized, setDirectoryNameCustomized] = useState(false);
  const [rootId, setRootId] = useState("");
  const [storageStrategy, setStorageStrategy] = useState<RepositoryStorageStrategy>("date");
  const [riskConfirmation, setRiskConfirmation] = useState(false);
  const [step, setStep] = useState<CreateRepositoryStep>(0);
  const nameError = validateRepositoryName(name);
  const nameErrorMessage = repositoryNameErrorMessage(nameError, t);
  const directoryNameError = validateRepositoryDirectoryName(directoryName);
  const directoryNameErrorMessage = repositoryDirectoryNameErrorMessage(directoryNameError, t);

  const roots = rootsQuery.data?.roots ?? [];
  const activeRoots = useMemo(
    () => roots.filter((root) => root.status === "active" && root.writable !== false),
    [roots],
  );
  const selectedRoot = useMemo(() => roots.find((root) => root.id === rootId), [rootId, roots]);
  const creatableCandidates = useMemo(
    () => (candidatesQuery.data?.candidates ?? []).filter((candidate) => candidate.can_create),
    [candidatesQuery.data?.candidates],
  );
  const selectedCandidate = useMemo(
    () => creatableCandidates.find((candidate) => candidate.directory_name === directoryName),
    [creatableCandidates, directoryName],
  );
  const placementRisks = useMemo(
    () =>
      Array.from(
        new Set([
          ...(selectedRoot?.risk_warnings ?? []),
          ...(selectedCandidate?.risk_warnings ?? []),
        ]),
      ),
    [selectedCandidate?.risk_warnings, selectedRoot?.risk_warnings],
  );

  useEffect(() => {
    if (!isOpen || activeRoots.length === 0) return;
    setRootId((current) => {
      if (activeRoots.some((root) => root.id === current)) return current;
      return activeRoots.find((root) => root.kind === "default")?.id ?? activeRoots[0]?.id ?? "";
    });
  }, [activeRoots, isOpen]);

  const handleClose = useCallback(() => {
    if (createRepositoryMutation.isPending) return;
    setName("");
    setDirectoryName("");
    setDirectoryNameCustomized(false);
    setRootId("");
    setStorageStrategy("date");
    setRiskConfirmation(false);
    setStep(0);
    onClose();
  }, [createRepositoryMutation.isPending, onClose]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (step < 3) {
        if (
          (step === 0 &&
            (validateRepositoryName(name) || validateRepositoryDirectoryName(directoryName))) ||
          (step === 1 && !rootId) ||
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
      if (!rootId) return;

      try {
        const response = await createRepositoryMutation.createRepository({
          name,
          directoryName,
          rootId,
          storageStrategy,
          riskConfirmation,
        });
        showMessage("success", t("manage.repositories.createSuccess", { name }));
        // The repository was created; these describe risks of where it landed,
        // such as a cloud-sync folder that may evict originals.
        for (const warning of response.warnings ?? []) {
          showMessage("info", warning);
        }
        setName("");
        setDirectoryName("");
        setDirectoryNameCustomized(false);
        setRootId("");
        setStorageStrategy("date");
        setRiskConfirmation(false);
        setStep(0);
        onClose();
      } catch (error) {
        const conflictType = repositoryConflictType(error);
        if (conflictType && onRecoveryRequired) {
          setName("");
          setDirectoryName("");
          setDirectoryNameCustomized(false);
          setRootId("");
          setStorageStrategy("date");
          setRiskConfirmation(false);
          setStep(0);
          onClose();
          onRecoveryRequired(conflictType);
          return;
        }
        showMessage(
          "error",
          error instanceof Error ? error.message : t("manage.repositories.createFailed"),
        );
      }
    },
    [
      createRepositoryMutation,
      directoryName,
      name,
      onClose,
      onRecoveryRequired,
      rootId,
      riskConfirmation,
      showMessage,
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
  const basicStepValid = nameError === null && directoryNameError === null;

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
                    {t("manage.repositories.createWizard.detailsTitle", "Name the repository")}
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
                      "Lumilio creates the repository as a direct child of this authorized location.",
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
                    value={rootId}
                    onChange={(event) => {
                      setRootId(event.target.value);
                      setRiskConfirmation(false);
                    }}
                    disabled={createRepositoryMutation.isPending || rootsQuery.isLoading}
                    required
                  >
                    {roots.length === 0 ? (
                      <option value="">
                        {rootsQuery.isLoading
                          ? t("manage.repositories.storageLocationLoading", "Loading locations…")
                          : t("manage.repositories.storageLocationEmpty", "No writable location")}
                      </option>
                    ) : null}
                    {roots.map((root) => (
                      <option
                        key={root.id}
                        value={root.id}
                        disabled={root.status !== "active" || root.writable === false}
                      >
                        {root.name}
                        {root.kind === "default"
                          ? ` · ${t("manage.repositories.storageLocationDefault", "Default")}`
                          : ""}
                        {root.status !== "active"
                          ? ` · ${t("manage.repositories.storageLocationOffline", "Offline")}`
                          : ""}
                        {root.status === "active" && root.writable === false
                          ? ` · ${t("manage.repositories.storageLocationReadOnly", "Read-only")}`
                          : ""}
                        {root.capacity_known
                          ? ` · ${formatBytes(root.available_bytes ?? 0)} ${t("manage.repositories.storageLocationAvailable", "available")}`
                          : ""}
                      </option>
                    ))}
                  </select>
                </div>
                {selectedRoot ? (
                  <div className="rounded-xl border border-base-300 bg-base-200/35 p-4">
                    <div className="flex items-start gap-3">
                      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-base-200 text-base-content/70">
                        <HardDrive size={18} />
                      </span>
                      <div className="min-w-0">
                        <div className="font-medium">{selectedRoot.name}</div>
                        <div className="mt-0.5 break-all font-mono text-xs text-base-content/55">
                          {selectedRoot.path}
                        </div>
                        <div className="mt-2 text-xs text-base-content/65">
                          {selectedRoot.writable
                            ? t("manage.repositories.storageLocationWritable", "Writable")
                            : t("manage.repositories.storageLocationReadOnly", "Read-only")}
                          {selectedRoot.capacity_known
                            ? ` · ${t("manage.repositories.storageLocationCapacity", "{{available}} of {{total}} available", { available: formatBytes(selectedRoot.available_bytes ?? 0), total: formatBytes(selectedRoot.total_bytes ?? 0) })}`
                            : ` · ${t("manage.repositories.storageLocationCapacityUnknown", "Capacity unavailable")}`}
                          {` · ${t("manage.repositories.storageLocationRepositoryCount", "{{count}} repositories", { count: selectedRoot.repository_count ?? 0 })}`}
                        </div>
                      </div>
                    </div>
                  </div>
                ) : null}
                <p className="text-xs leading-snug text-base-content/55">
                  {t(
                    "manage.repositories.storageLocationHint",
                    "External locations are authorized in the Desktop Control Panel.",
                  )}
                </p>
                {!rootsQuery.isLoading && activeRoots.length === 0 && canRequestStorageLocation ? (
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
                {rootsQuery.isError ? (
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
                      "Confirm the repository identity and its permanent storage layout before creating it.",
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
                    value={selectedRoot?.name || "—"}
                    detail={selectedRoot?.path}
                  />
                  <ReviewRow
                    label={t("manage.repositories.storageStrategy.label", "Storage layout")}
                    value={storageStrategyLabel(storageStrategy, t)}
                    icon={<LayoutGrid size={15} />}
                  />
                </dl>
                <StorageRiskConfirmation
                  warnings={placementRisks}
                  checked={riskConfirmation}
                  onChange={setRiskConfirmation}
                  disabled={createRepositoryMutation.isPending}
                />
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
                    (step === 1 && !rootId) ||
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
                    (placementRisks.length > 0 && !riskConfirmation) ||
                    createRepositoryMutation.isPending
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

function repositoryConflictType(error: unknown): string | null {
  if (!error || typeof error !== "object" || !("conflict_type" in error)) return null;
  return typeof error.conflict_type === "string" ? error.conflict_type : null;
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
      return t("manage.repositories.createNameRequired", "Enter a repository name.");
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
