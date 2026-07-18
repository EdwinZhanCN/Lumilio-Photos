import React, { FormEvent, useEffect, useMemo, useState } from "react";
import { FolderPlus, HardDrive } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { setupStatusQueryKey, useSetupStatus } from "../api/useSetupStatus.ts";

const isStorageStrategy = (value?: string): value is "cas" | "date" | "flat" =>
  value === "cas" || value === "date" || value === "flat";
const isDuplicateHandling = (value?: string): value is "overwrite" | "rename" | "uuid" =>
  value === "overwrite" || value === "rename" || value === "uuid";

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
  const createMutation = $api.useMutation("post", "/api/v1/repositories");
  const defaults = setupQuery.data?.repository_defaults;
  const primaryReady = setupQuery.data?.primary_repository_initialized ?? false;
  const [name, setName] = useState("Primary Storage");
  const [root, setRoot] = useState("");
  const [strategy, setStrategy] = useState<"cas" | "date" | "flat">("date");
  const [duplicateHandling, setDuplicateHandling] = useState<"overwrite" | "rename" | "uuid">(
    "rename",
  );

  useEffect(() => {
    if (!defaults) return;
    setRoot((current) => current || defaults.default_root || "");
    setStrategy(isStorageStrategy(defaults.strategy) ? defaults.strategy : "date");
    setDuplicateHandling(
      isDuplicateHandling(defaults.duplicate_handling) ? defaults.duplicate_handling : "rename",
    );
  }, [defaults]);

  const canSubmit = useMemo(
    () => name.trim() !== "" && root.trim() !== "" && !createMutation.isPending,
    [createMutation.isPending, name, root],
  );

  if (setupQuery.isLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-base-200">
        <div className="flex flex-col items-center gap-4">
          <span className="loading loading-spinner loading-lg text-primary" />
          <p className="animate-pulse text-sm font-medium opacity-50">
            {t("auth.primaryRepository.loading", {
              defaultValue: "Checking repository setup...",
            })}
          </p>
        </div>
      </div>
    );
  }

  if (primaryReady) {
    return <>{children}</>;
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit) return;
    await createMutation.mutateAsync({
      body: {
        name: name.trim(),
        role: "primary",
        storage_strategy: strategy,
        duplicate_handling: duplicateHandling,
      },
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
          defaultValue: "Failed to create the primary repository.",
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
                defaultValue: "Create primary repository",
              })}
            </h1>
            <p className="mt-1 text-sm text-base-content/70">
              {t("auth.primaryRepository.description", {
                defaultValue:
                  "Choose where Lumilio should store the first local repository. Existing settings use this as the default for future repositories too.",
              })}
            </p>
          </div>
        </div>

        {setupQuery.isError && (
          <div className="alert alert-error mb-4 text-sm">
            {t("auth.primaryRepository.statusError", {
              defaultValue: "Unable to load repository defaults.",
            })}
          </div>
        )}

        {error && <div className="alert alert-error mb-4 text-sm">{error}</div>}

        <form onSubmit={submit} className="space-y-4">
          <label className="form-control">
            <span className="label-text mb-1 font-medium">
              {t("auth.primaryRepository.name", { defaultValue: "Name" })}
            </span>
            <input
              className="input input-bordered w-full"
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={createMutation.isPending}
              required
            />
          </label>

          <label className="form-control">
            <span className="label-text mb-1 font-medium">
              {t("auth.primaryRepository.root", {
                defaultValue: "Storage root",
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
                  "Set by server configuration. The primary repository is created at <root>/primary.",
              })}
            </span>
          </label>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="form-control">
              <span className="label-text mb-1 font-medium">
                {t("auth.primaryRepository.strategy", { defaultValue: "Storage strategy" })}
              </span>
              <select
                className="select select-bordered w-full"
                value={strategy}
                onChange={(event) => {
                  if (isStorageStrategy(event.target.value)) setStrategy(event.target.value);
                }}
                disabled={createMutation.isPending}
              >
                <option value="date">date</option>
                <option value="flat">flat</option>
                <option value="cas">cas</option>
              </select>
            </label>

            <label className="form-control">
              <span className="label-text mb-1 font-medium">
                {t("auth.primaryRepository.duplicates", { defaultValue: "Duplicates" })}
              </span>
              <select
                className="select select-bordered w-full"
                value={duplicateHandling}
                onChange={(event) => {
                  if (isDuplicateHandling(event.target.value)) {
                    setDuplicateHandling(event.target.value);
                  }
                }}
                disabled={createMutation.isPending}
              >
                <option value="rename">rename</option>
                <option value="uuid">uuid</option>
                <option value="overwrite">overwrite</option>
              </select>
            </label>
          </div>

          <div className="flex justify-end pt-2">
            <button type="submit" className="btn btn-primary gap-2" disabled={!canSubmit}>
              {createMutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <FolderPlus size={16} />
              )}
              {t("auth.primaryRepository.submit", { defaultValue: "Create repository" })}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default PrimaryRepositoryGate;
