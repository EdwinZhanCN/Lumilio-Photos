import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Cloud, FolderPlus, X } from "lucide-react";
import { useCloudCredentials, createProviderTextResolver } from "@/features/cloud";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n.tsx";
import { useCreateRepository } from "../../api/useCreateRepository";
import { useRepositoryRoots } from "../../api/useRepositoryRoots";
import {
  isDuplicateHandling,
  isStorageStrategy,
  type DuplicateHandling,
  type StorageStrategy,
} from "../../model/repositorySetup";

export default function AddRepositoryModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const providerText = createProviderTextResolver(t);
  const showMessage = useMessage();
  const createRepositoryMutation = useCreateRepository();
  const credentialsQuery = useCloudCredentials();
  const rootsQuery = useRepositoryRoots();
  const [name, setName] = useState("");
  const [source, setSource] = useState<"local" | "cloud">("local");
  const [credentialId, setCredentialId] = useState("");
  const [rootId, setRootId] = useState("");
  const [storageStrategy, setStorageStrategy] = useState<StorageStrategy>("date");
  const [duplicateHandling, setDuplicateHandling] = useState<DuplicateHandling>("rename");

  const credentials = useMemo(
    () => (credentialsQuery.data?.credentials ?? []).filter((item) => item.status === "connected"),
    [credentialsQuery.data],
  );
  const roots = rootsQuery.data?.roots ?? [];
  const activeRoots = useMemo(() => roots.filter((root) => root.status === "active"), [roots]);

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
    setSource("local");
    setCredentialId("");
    setRootId("");
    setStorageStrategy("date");
    setDuplicateHandling("rename");
    onClose();
  }, [createRepositoryMutation.isPending, onClose]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const trimmedName = name.trim();
      if (!trimmedName || createRepositoryMutation.isPending) return;
      if (!rootId) return;
      if (source === "cloud" && !credentialId) return;

      try {
        const response = await createRepositoryMutation.createRepository({
          name: trimmedName,
          rootId,
          cloudCredentialId: source === "cloud" ? credentialId : undefined,
          storageStrategy,
          duplicateHandling,
        });
        showMessage(
          response.cloud_import_error ? "info" : "success",
          response.cloud_import_error
            ? t("manage.repositories.cloudImportCreatePartial", {
                error: response.cloud_import_error,
              })
            : source === "cloud"
              ? t("manage.repositories.cloudImportCreateSuccess", {
                  name: trimmedName,
                })
              : t("manage.repositories.createSuccess", { name: trimmedName }),
        );
        // The repository was created; these describe risks of where it landed,
        // such as a cloud-sync folder that may evict originals.
        for (const warning of response.warnings ?? []) {
          showMessage("info", warning);
        }
        setName("");
        setSource("local");
        setCredentialId("");
        setRootId("");
        setStorageStrategy("date");
        setDuplicateHandling("rename");
        onClose();
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error ? error.message : t("manage.repositories.createFailed"),
        );
      }
    },
    [
      createRepositoryMutation,
      credentialId,
      duplicateHandling,
      name,
      onClose,
      rootId,
      showMessage,
      source,
      storageStrategy,
      t,
    ],
  );

  if (!isOpen) return null;

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-md">
        <div className="mb-5 flex items-start justify-between gap-4">
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

        <form onSubmit={handleSubmit} className="space-y-5">
          <label className="form-control w-full">
            <span className="label pb-1">
              <span className="label-text font-medium">
                {t("manage.repositories.createNameLabel")}
              </span>
            </span>
            <input
              type="text"
              className="input input-bordered w-full"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("manage.repositories.createNamePlaceholder")}
              disabled={createRepositoryMutation.isPending}
              autoFocus
              required
            />
          </label>

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
              onChange={(event) => setRootId(event.target.value)}
              disabled={createRepositoryMutation.isPending || rootsQuery.isLoading}
              required
            >
              {roots.length === 0 && (
                <option value="">
                  {rootsQuery.isLoading
                    ? t("manage.repositories.storageLocationLoading", "Loading locations…")
                    : t("manage.repositories.storageLocationEmpty", "No writable location")}
                </option>
              )}
              {roots.map((root) => (
                <option key={root.id} value={root.id} disabled={root.status !== "active"}>
                  {root.name}
                  {root.kind === "default"
                    ? ` · ${t("manage.repositories.storageLocationDefault", "Default")}`
                    : ""}
                  {root.status !== "active"
                    ? ` · ${t("manage.repositories.storageLocationOffline", "Offline")}`
                    : ""}
                </option>
              ))}
            </select>
            <span className="label text-xs leading-snug text-base-content/55">
              {t(
                "manage.repositories.storageLocationHint",
                "External locations are authorized in the Desktop Control Panel.",
              )}
            </span>
            {rootsQuery.isError && (
              <div role="alert" className="alert alert-error alert-soft mt-2 py-2 text-xs">
                {t(
                  "manage.repositories.storageLocationError",
                  "Storage Locations are unavailable.",
                )}
              </div>
            )}
          </div>

          <div className="space-y-2">
            <span className="text-sm font-medium">{t("manage.repositories.sourceLabel")}</span>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                className={`btn btn-sm ${source === "local" ? "btn-primary" : "btn-outline"}`}
                onClick={() => setSource("local")}
                disabled={createRepositoryMutation.isPending}
              >
                {t("manage.repositories.sourceLocal")}
              </button>
              <button
                type="button"
                className={`btn btn-sm gap-2 ${source === "cloud" ? "btn-primary" : "btn-outline"}`}
                onClick={() => setSource("cloud")}
                disabled={createRepositoryMutation.isPending}
              >
                <Cloud size={15} />
                {t("manage.repositories.sourceCloud")}
              </button>
            </div>
          </div>

          {source === "cloud" && (
            <div className="form-control w-full">
              <label className="label pb-1" htmlFor="repository-cloud-credential">
                <span className="label-text font-medium">
                  {t("manage.repositories.cloudCredentialLabel")}
                </span>
              </label>
              <select
                id="repository-cloud-credential"
                className="select select-bordered w-full"
                value={credentialId}
                onChange={(event) => setCredentialId(event.target.value)}
                disabled={createRepositoryMutation.isPending || credentialsQuery.isLoading}
                required
              >
                <option value="">
                  {credentials.length === 0
                    ? t("manage.repositories.noCloudCredentials")
                    : t("manage.repositories.selectCloudCredential")}
                </option>
                {credentials.map((credential) => (
                  <option key={credential.id} value={credential.id}>
                    {credential.display_name} ·{" "}
                    {providerText(credential.provider_title) || credential.provider} ·{" "}
                    {credential.masked_identity}
                  </option>
                ))}
              </select>
              <p className="mt-2 text-xs leading-relaxed text-base-content/60">
                {t("manage.repositories.cloudCredentialsHintPrefix")}{" "}
                <Link
                  to="/settings?tab=cloud"
                  className="link link-primary font-medium"
                  onClick={handleClose}
                >
                  {t("manage.repositories.cloudCredentialsHintLink")}
                </Link>
                {t("manage.repositories.cloudCredentialsHintSuffix")}
              </p>
            </div>
          )}

          <fieldset className="fieldset grid gap-4 rounded-lg border border-base-300 px-4 pb-4">
            <legend className="fieldset-legend px-1">
              {t("manage.repositories.policyLegend", "Repository policy")}
            </legend>

            <div className="fieldset min-w-0 gap-1">
              <label
                className="fieldset-legend p-0 text-xs font-medium"
                htmlFor="repository-storage-strategy"
              >
                {t("manage.repositories.storageStrategyLabel", "File layout")}
              </label>
              <select
                id="repository-storage-strategy"
                className="select select-bordered select-sm w-full"
                value={storageStrategy}
                onChange={(event) => {
                  if (isStorageStrategy(event.target.value)) {
                    setStorageStrategy(event.target.value);
                  }
                }}
                disabled={createRepositoryMutation.isPending}
              >
                <option value="date">
                  {t("manage.repositories.storageStrategyDate", "By capture date")}
                </option>
                <option value="flat">
                  {t("manage.repositories.storageStrategyFlat", "Single folder")}
                </option>
                <option value="cas">
                  {t("manage.repositories.storageStrategyCas", "Content addressed")}
                </option>
              </select>
              <span className="label text-[0.7rem] leading-snug text-base-content/55">
                {t(
                  "manage.repositories.storageStrategyHint",
                  "Controls where imported originals are placed.",
                )}
              </span>
            </div>

            <div className="fieldset min-w-0 gap-1">
              <label
                className="fieldset-legend p-0 text-xs font-medium"
                htmlFor="repository-duplicate-handling"
              >
                {t("manage.repositories.duplicateHandlingLabel", "Filename conflicts")}
              </label>
              <select
                id="repository-duplicate-handling"
                className="select select-bordered select-sm w-full"
                value={duplicateHandling}
                onChange={(event) => {
                  if (isDuplicateHandling(event.target.value)) {
                    setDuplicateHandling(event.target.value);
                  }
                }}
                disabled={createRepositoryMutation.isPending}
              >
                <option value="rename">
                  {t("manage.repositories.duplicateHandlingRename", "Keep both (rename)")}
                </option>
                <option value="uuid">
                  {t("manage.repositories.duplicateHandlingUuid", "Keep both (unique ID)")}
                </option>
                <option value="overwrite">
                  {t("manage.repositories.duplicateHandlingOverwrite", "Overwrite existing")}
                </option>
              </select>
              <span className="label text-[0.7rem] leading-snug text-base-content/55">
                {t(
                  "manage.repositories.duplicateHandlingHint",
                  "Applied when two imported files resolve to the same name.",
                )}
              </span>
            </div>
          </fieldset>

          <div className="modal-action">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={handleClose}
              disabled={createRepositoryMutation.isPending}
            >
              {t("common.cancel", { defaultValue: "Cancel" })}
            </button>
            <button
              type="submit"
              className="btn btn-primary gap-2"
              disabled={
                !name.trim() ||
                !rootId ||
                createRepositoryMutation.isPending ||
                (source === "cloud" && !credentialId)
              }
            >
              {createRepositoryMutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <FolderPlus size={16} />
              )}
              {t("manage.repositories.createSubmit")}
            </button>
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
