import React, { FormEvent, useEffect, useMemo, useState } from "react";
import { AlertTriangle, FolderPlus, HardDrive, RefreshCw } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import {
  StorageStrategyPicker,
  StorageRiskConfirmation,
  useCreateRepository,
  validateRepositoryName,
  type RepositoryStorageStrategy,
} from "@/features/repositories";
import { useI18n } from "@/lib/i18n.tsx";
import { setupStatusQueryKey, useSetupStatus } from "../../api/useSetupStatus.ts";
import { repositoryNameErrorMessage } from "../../model/repositoryNameValidation.ts";

function apiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const record = error as { message?: string; error?: string };
    return record.message || record.error || fallback;
  }
  return fallback;
}

const PrimaryRepositoryGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const setupQuery = useSetupStatus();
  const createMutation = useCreateRepository();
  const defaults = setupQuery.data?.repository_defaults;
  const primaryReady = setupQuery.data?.primary_repository_initialized ?? false;
  const [name, setName] = useState("Primary Storage");
  const [root, setRoot] = useState("");
  const [storageStrategy, setStorageStrategy] = useState<RepositoryStorageStrategy>("date");
  const [riskConfirmation, setRiskConfirmation] = useState(false);
  const placementRisks = defaults?.risk_warnings ?? [];

  useEffect(() => {
    if (!defaults) return;
    setRoot((current) => current || defaults.default_root || "");
  }, [defaults]);

  const nameError = validateRepositoryName(name);
  const canSubmit = useMemo(
    () =>
      nameError === null &&
      root.trim() !== "" &&
      (placementRisks.length === 0 || riskConfirmation) &&
      !createMutation.isPending,
    [createMutation.isPending, nameError, placementRisks.length, riskConfirmation, root],
  );

  if (setupQuery.isLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-base-200">
        <div className="flex flex-col items-center gap-4">
          <span className="loading loading-spinner loading-lg text-primary" />
          <p className="animate-pulse text-sm font-medium opacity-50">
            {t("auth.primaryRepository.loading", {
              defaultValue: "Checking Primary Repository setup...",
            })}
          </p>
        </div>
      </div>
    );
  }

  if (primaryReady) {
    if (setupQuery.data?.runtime_state === "degraded") {
      return (
        <>
          <div
            role="alert"
            className="alert alert-warning rounded-none border-x-0 border-t-0 px-4 py-3"
          >
            <AlertTriangle className="size-5 shrink-0" />
            <div className="min-w-0 flex-1">
              <strong>{t("auth.storageRecovery.title", "Storage recovery is required")}</strong>
              <p className="mt-0.5 text-sm">
                {t(
                  "auth.storageRecovery.description",
                  "Restore the configured Default Storage Location and its Primary Repository without changing either marker identity. Other available Repositories remain usable.",
                )}
              </p>
              {defaults?.default_root ? (
                <code className="mt-1 block truncate text-xs">{defaults.default_root}/primary</code>
              ) : null}
            </div>
            <button
              type="button"
              className="btn btn-sm btn-ghost gap-2"
              onClick={() => void setupQuery.refetch()}
              disabled={setupQuery.isFetching}
            >
              <RefreshCw className={`size-4 ${setupQuery.isFetching ? "animate-spin" : ""}`} />
              {t("auth.storageRecovery.checkAgain", "Check Again")}
            </button>
          </div>
          {children}
        </>
      );
    }
    return <>{children}</>;
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit) return;
    await createMutation.createRepository({
      name,
      role: "primary",
      storageStrategy,
      riskConfirmation: placementRisks.length > 0 ? riskConfirmation : undefined,
    });
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: setupStatusQueryKey }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/indexing/repositories"] }),
    ]);
  };

  const error = createMutation.error
    ? apiMessage(
        createMutation.error,
        t("auth.primaryRepository.error", {
          defaultValue: "Failed to create the Primary Repository.",
        }),
      )
    : null;

  return (
    <div className="flex min-h-dvh items-center justify-center bg-base-200 px-4">
      <div className="w-full max-w-xl rounded-lg border border-base-300 bg-base-100 p-6 shadow-xl">
        <div className="mb-6 flex items-start gap-4">
          <div className="flex size-11 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <HardDrive size={22} />
          </div>
          <div>
            <h1 className="text-xl font-semibold">
              {t("auth.primaryRepository.title", {
                defaultValue: "Create Primary Repository",
              })}
            </h1>
            <p className="mt-1 text-sm text-base-content/70">
              {t("auth.primaryRepository.description", {
                defaultValue:
                  "The Primary Repository is the unique Repository in the Default Storage Location's primary/ folder. Lumilio uses it when no other Repository is selected.",
              })}
            </p>
          </div>
        </div>

        {setupQuery.isError && (
          <div className="alert alert-error mb-4 text-sm">
            {t("auth.primaryRepository.statusError", {
              defaultValue: "Unable to load Primary Repository defaults.",
            })}
          </div>
        )}

        {error && <div className="alert alert-error mb-4 text-sm">{error}</div>}

        <form onSubmit={submit} className="space-y-4">
          <div className="fieldset gap-1">
            <label className="fieldset-legend p-0 font-medium" htmlFor="primary-repository-name">
              {t("auth.primaryRepository.name", { defaultValue: "Name" })}
            </label>
            <input
              id="primary-repository-name"
              className={`input input-bordered w-full ${
                name.length > 0 && nameError ? "input-error" : ""
              }`}
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={createMutation.isPending}
              aria-invalid={name.length > 0 && nameError !== null}
              aria-describedby="primary-repository-name-hint"
              required
            />
            <span
              id="primary-repository-name-hint"
              className={`label text-xs leading-snug ${
                name.length > 0 && nameError ? "text-error" : "text-base-content/55"
              }`}
            >
              {name.length > 0 && nameError
                ? repositoryNameErrorMessage(nameError, t)
                : t(
                    "auth.primaryRepository.nameHint",
                    "This display name can be changed later. The Primary Repository directory remains primary/.",
                  )}
            </span>
          </div>

          <label className="form-control">
            <span className="label-text mb-1 font-medium">
              {t("auth.primaryRepository.root", {
                defaultValue: "Default Storage Location",
              })}
            </span>
            <input
              className="input input-bordered w-full bg-base-200 font-mono text-sm"
              value={root}
              readOnly
              tabIndex={-1}
            />
            <span className="label-text-alt mt-1 text-base-content/50">
              {t("auth.primaryRepository.rootHint", {
                defaultValue:
                  "Set by server configuration. Lumilio creates the Primary Repository in this Default Storage Location's primary/ folder.",
              })}
            </span>
          </label>

          <StorageStrategyPicker
            value={storageStrategy}
            onChange={setStorageStrategy}
            disabled={createMutation.isPending}
            idPrefix="primary-gate"
          />

          <StorageRiskConfirmation
            warnings={placementRisks}
            checked={riskConfirmation}
            onChange={setRiskConfirmation}
            disabled={createMutation.isPending}
          />

          <div className="flex justify-end pt-2">
            <button type="submit" className="btn btn-primary gap-2" disabled={!canSubmit}>
              {createMutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <FolderPlus size={16} />
              )}
              {t("auth.primaryRepository.submit", {
                defaultValue: "Create Primary Repository",
              })}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default PrimaryRepositoryGate;
