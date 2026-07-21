import { useCallback, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Cloud, FolderPlus, X } from "lucide-react";
import { useCloudCredentials } from "@/features/cloud";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n.tsx";
import { useCreateRepository } from "../../api/useCreateRepository";

export default function AddRepositoryModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const createRepositoryMutation = useCreateRepository();
  const credentialsQuery = useCloudCredentials();
  const [name, setName] = useState("");
  const [source, setSource] = useState<"local" | "cloud">("local");
  const [credentialId, setCredentialId] = useState("");

  const credentials = useMemo(
    () => (credentialsQuery.data?.credentials ?? []).filter((item) => item.status === "connected"),
    [credentialsQuery.data],
  );

  const handleClose = useCallback(() => {
    if (createRepositoryMutation.isPending) return;
    setName("");
    setSource("local");
    setCredentialId("");
    onClose();
  }, [createRepositoryMutation.isPending, onClose]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const trimmedName = name.trim();
      if (!trimmedName || createRepositoryMutation.isPending) return;
      if (source === "cloud" && !credentialId) return;

      try {
        const response = await createRepositoryMutation.createRepository({
          name: trimmedName,
          cloudCredentialId: source === "cloud" ? credentialId : undefined,
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
        onClose();
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error ? error.message : t("manage.repositories.createFailed"),
        );
      }
    },
    [createRepositoryMutation, credentialId, name, onClose, showMessage, source, t],
  );

  if (!isOpen) return null;

  return (
    <div className="modal modal-open z-50">
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
                    {credential.display_name} · {credential.provider_title ?? credential.provider} ·{" "}
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
