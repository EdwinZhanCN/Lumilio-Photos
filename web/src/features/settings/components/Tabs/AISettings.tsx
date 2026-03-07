import { useEffect, useMemo, useState } from "react";
import { SparklesIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useSystemSettings,
  useUpdateSystemSettings,
  useValidateLLMSettings,
  type SystemSettings,
  type UpdateSystemSettingsPayload,
} from "@/features/settings/hooks/useSystemSettings";
import { useCapabilities } from "@/features/settings/hooks/useCapabilities";
import {
  useAssetIndexingStats,
  useRebuildAssetIndexes,
} from "@/features/settings/hooks/useAssetIndexing";
import { useWorkingRepository } from "@/features/settings/hooks/useWorkingRepository";

type AgentProvider = SystemSettings["llm"]["provider"];
type AutoMode = SystemSettings["ml"]["autoMode"];

type AIFormState = {
  llm: {
    agentEnabled: boolean;
    provider: AgentProvider;
    modelName: string;
    baseURL: string;
    apiKey: string;
    clearStoredKey: boolean;
  };
  ml: {
    autoMode: AutoMode;
    clipEnabled: boolean;
    ocrEnabled: boolean;
    captionEnabled: boolean;
    faceEnabled: boolean;
  };
};

type FeedbackState = {
  tone: "success" | "error";
  message: string;
} | null;

type RuntimeCapability = {
  enabled: boolean;
  available: boolean;
};

type ReindexTaskId = "clip" | "ocr" | "caption" | "face";

function configuredBadgeClass(enabled: boolean): string {
  return enabled ? "badge badge-success badge-outline" : "badge badge-ghost";
}

function runtimeBadgeClass(capability?: RuntimeCapability): string {
  if (!capability?.enabled) {
    return "badge badge-ghost";
  }

  return capability.available
    ? "badge badge-success badge-outline"
    : "badge badge-warning badge-outline";
}

function coverageBadgeClass(coverage: number): string {
  if (coverage >= 0.9) {
    return "badge badge-success badge-outline";
  }
  if (coverage > 0) {
    return "badge badge-warning badge-outline";
  }
  return "badge badge-ghost";
}

function formatCoveragePercent(coverage: number): string {
  return `${Math.round(coverage * 100)}%`;
}

function deriveDefaultReindexTasks(form: AIFormState): ReindexTaskId[] {
  if (form.ml.autoMode === "enable") {
    return ["clip", "ocr", "caption", "face"];
  }

  const tasks: ReindexTaskId[] = [];
  if (form.ml.clipEnabled) tasks.push("clip");
  if (form.ml.ocrEnabled) tasks.push("ocr");
  if (form.ml.captionEnabled) tasks.push("caption");
  if (form.ml.faceEnabled) tasks.push("face");
  return tasks;
}

function createFormState(settings: SystemSettings): AIFormState {
  return {
    llm: {
      agentEnabled: settings.llm.agentEnabled,
      provider: settings.llm.provider,
      modelName: settings.llm.modelName,
      baseURL: settings.llm.baseURL,
      apiKey: "",
      clearStoredKey: false,
    },
    ml: {
      autoMode: settings.ml.autoMode,
      clipEnabled: settings.ml.clipEnabled,
      ocrEnabled: settings.ml.ocrEnabled,
      captionEnabled: settings.ml.captionEnabled,
      faceEnabled: settings.ml.faceEnabled,
    },
  };
}

function buildPayload(form: AIFormState): UpdateSystemSettingsPayload {
  const payload: UpdateSystemSettingsPayload = {
    llm: {
      agent_enabled: form.llm.agentEnabled,
      model_name: form.llm.modelName.trim(),
      base_url: form.llm.baseURL.trim(),
    },
    ml: {
      auto_mode: form.ml.autoMode,
      clip_enabled: form.ml.clipEnabled,
      ocr_enabled: form.ml.ocrEnabled,
      caption_enabled: form.ml.captionEnabled,
      face_enabled: form.ml.faceEnabled,
    },
  };

  if (form.llm.provider) {
    payload.llm = {
      ...payload.llm,
      provider: form.llm.provider,
    };
  }

  if (form.llm.clearStoredKey) {
    payload.llm = {
      ...payload.llm,
      api_key: "",
    };
  } else if (form.llm.apiKey.trim()) {
    payload.llm = {
      ...payload.llm,
      api_key: form.llm.apiKey.trim(),
    };
  }

  return payload;
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (error && typeof error === "object") {
    const maybeApiError = error as { message?: string; error?: string };
    if (maybeApiError.message) {
      return maybeApiError.message;
    }
    if (maybeApiError.error) {
      return maybeApiError.error;
    }
  }
  return fallback;
}

export default function AISettings() {
  const { t } = useI18n();
  const [selectedReindexTasks, setSelectedReindexTasks] = useState<
    ReindexTaskId[]
  >([]);
  const settingsQuery = useSystemSettings();
  const updateMutation = useUpdateSystemSettings();
  const validateMutation = useValidateLLMSettings();
  const capabilitiesQuery = useCapabilities();
  const { scopeLabel, scopeDescription, scopedRepositoryId } =
    useWorkingRepository();
  const indexingStatsQuery = useAssetIndexingStats(scopedRepositoryId);
  const rebuildMutation = useRebuildAssetIndexes();
  const settings = settingsQuery.settings;
  const capabilities = capabilitiesQuery.capabilities;
  const indexingStats = indexingStatsQuery.stats;
  const [form, setForm] = useState<AIFormState | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState>(null);

  useEffect(() => {
    if (!settings) return;
    setForm(createFormState(settings));
  }, [
    settings?.llm.agentEnabled,
    settings?.llm.provider,
    settings?.llm.modelName,
    settings?.llm.baseURL,
    settings?.llm.apiKeyConfigured,
    settings?.ml.autoMode,
    settings?.ml.clipEnabled,
    settings?.ml.ocrEnabled,
    settings?.ml.captionEnabled,
    settings?.ml.faceEnabled,
  ]);

  useEffect(() => {
    if (!form) {
      return;
    }

    setSelectedReindexTasks(deriveDefaultReindexTasks(form));
  }, [
    form?.ml.autoMode,
    form?.ml.clipEnabled,
    form?.ml.ocrEnabled,
    form?.ml.captionEnabled,
    form?.ml.faceEnabled,
  ]);

  const isDirty = useMemo(() => {
    if (!settings || !form) {
      return false;
    }

    return (
      form.llm.agentEnabled !== settings.llm.agentEnabled ||
      form.llm.provider !== settings.llm.provider ||
      form.llm.modelName !== settings.llm.modelName ||
      form.llm.baseURL !== settings.llm.baseURL ||
      form.llm.apiKey.trim().length > 0 ||
      form.llm.clearStoredKey ||
      form.ml.autoMode !== settings.ml.autoMode ||
      form.ml.clipEnabled !== settings.ml.clipEnabled ||
      form.ml.ocrEnabled !== settings.ml.ocrEnabled ||
      form.ml.captionEnabled !== settings.ml.captionEnabled ||
      form.ml.faceEnabled !== settings.ml.faceEnabled
    );
  }, [form, settings]);

  const isBusy = updateMutation.isPending || validateMutation.isPending;
  const semanticSearchRuntimeReady = Boolean(
    capabilities?.ml.tasks.clipImageEmbed.available &&
      capabilities.ml.tasks.clipTextEmbed.available,
  );

  const runtimeAvailabilityLabel = (capability?: RuntimeCapability): string => {
    if (!capability) {
      if (capabilitiesQuery.isLoading) {
        return t("common.loading");
      }

      return t("common.na");
    }

    return capability.available
      ? t("settings.serverSettings.available")
      : t("settings.serverSettings.unavailable");
  };

  const mlTasks = [
    {
      statsKey: "clip",
      key: "clipEnabled",
      label: t("settings.aiSettings.taskNames.clip"),
      description: t("settings.aiSettings.taskDescriptions.clip"),
      runtime: capabilities?.ml.tasks.clipImageEmbed,
      queryRuntime: capabilities?.ml.tasks.clipTextEmbed,
    },
    {
      statsKey: "ocr",
      key: "ocrEnabled",
      label: t("settings.aiSettings.taskNames.ocr"),
      description: t("settings.aiSettings.taskDescriptions.ocr"),
      runtime: capabilities?.ml.tasks.ocr,
      queryRuntime: undefined,
    },
    {
      statsKey: "caption",
      key: "captionEnabled",
      label: t("settings.aiSettings.taskNames.caption"),
      description: t("settings.aiSettings.taskDescriptions.caption"),
      runtime: capabilities?.ml.tasks.vlmGenerate,
      queryRuntime: undefined,
    },
    {
      statsKey: "face",
      key: "faceEnabled",
      label: t("settings.aiSettings.taskNames.face"),
      description: t("settings.aiSettings.taskDescriptions.face"),
      runtime: capabilities?.ml.tasks.faceDetectAndEmbed,
      queryRuntime: undefined,
    },
  ] as const;

  const persistSettings = async (): Promise<boolean> => {
    if (!form) {
      return false;
    }

    try {
      await updateMutation.mutateAsync({
        body: buildPayload(form),
      });

      setForm((current) =>
        current
          ? {
              ...current,
              llm: {
                ...current.llm,
                apiKey: "",
                clearStoredKey: false,
              },
            }
          : current,
      );
      setFeedback({
        tone: "success",
        message: t("settings.aiSettings.saveSuccess"),
      });
      return true;
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(error, t("settings.aiSettings.saveError")),
      });
      return false;
    }
  };

  const handleValidate = async () => {
    if (!form) return;

    const ready = !isDirty || (await persistSettings());
    if (!ready) {
      return;
    }

    try {
      await validateMutation.mutateAsync({});
      setFeedback({
        tone: "success",
        message: t("settings.aiSettings.validationSuccess"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.aiSettings.validationError"),
        ),
      });
    }
  };

  const handleQueueReindex = async () => {
    if (isDirty) {
      setFeedback({
        tone: "error",
        message: t("settings.aiSettings.saveBeforeReindexHint"),
      });
      return;
    }
    if (selectedReindexTasks.length === 0) {
      setFeedback({
        tone: "error",
        message: t("settings.aiSettings.selectAtLeastOneTask"),
      });
      return;
    }

    try {
      await rebuildMutation.mutateAsync({
        body: {
          missing_only: true,
          limit: 200,
          repository_id: scopedRepositoryId,
          tasks: selectedReindexTasks,
        },
      });
      setFeedback({
        tone: "success",
        message: t("settings.aiSettings.reindexQueuedSuccess"),
      });
      await indexingStatsQuery.refetch();
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.aiSettings.reindexQueuedError"),
        ),
      });
    }
  };

  const toggleReindexTask = (task: ReindexTaskId, checked: boolean) => {
    setSelectedReindexTasks((current) => {
      if (checked) {
        return Array.from(new Set([...current, task])) as ReindexTaskId[];
      }
      return current.filter((value) => value !== task);
    });
  };

  if (settingsQuery.isLoading || !form) {
    return (
      <div className="rounded-2xl border border-base-300 bg-base-100 px-4 py-6 text-sm text-base-content/70">
        {t("common.loading")}
      </div>
    );
  }

  if (settingsQuery.isError || !settings) {
    return (
      <div className="alert alert-warning">
        <span>{t("settings.aiSettings.loadError")}</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <SparklesIcon className="size-6 text-primary" />
          <h2 className="text-2xl font-bold">{t("settings.ai")}</h2>
        </div>
        <p className="text-base-content/70">
          {t("settings.aiSettings.description")}
        </p>
      </header>

      {feedback && (
        <div
          className={`alert ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
        >
          <span>{feedback.message}</span>
        </div>
      )}

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-5">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold">
              {t("settings.aiSettings.agentTitle")}
            </h3>
            <p className="text-sm text-base-content/70">
              {t("settings.aiSettings.agentDescription")}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="badge badge-outline">
              {t("settings.aiSettings.apiKeyConfigured")}:{" "}
              {t(
                `settings.serverSettings.booleanValues.${settings.llm.apiKeyConfigured ? "true" : "false"}`,
              )}
            </span>
          </div>
        </div>

        <div className="rounded-2xl border border-base-300 bg-base-100 p-4 flex items-start justify-between gap-4">
          <div>
            <div className="font-semibold">
              {t("settings.aiSettings.enabled")}
            </div>
          </div>
          <input
            type="checkbox"
            className="toggle toggle-primary"
            checked={form.llm.agentEnabled}
            onChange={(event) => {
              setFeedback(null);
              setForm((current) =>
                current
                  ? {
                      ...current,
                      llm: {
                        ...current.llm,
                        agentEnabled: event.target.checked,
                      },
                    }
                  : current,
              );
            }}
          />
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <label className="form-control gap-2">
            <span className="font-semibold">
              {t("settings.aiSettings.provider")}
            </span>
            <select
              className="select select-bordered w-full"
              value={form.llm.provider}
              onChange={(event) => {
                const provider = event.target.value as AgentProvider;
                setFeedback(null);
                setForm((current) =>
                  current
                    ? {
                        ...current,
                        llm: {
                          ...current.llm,
                          provider,
                        },
                      }
                    : current,
                );
              }}
            >
              <option value="">
                {t("settings.aiSettings.providerOptions.unset")}
              </option>
              <option value="ark">
                {t("settings.aiSettings.providerOptions.ark")}
              </option>
              <option value="openai">
                {t("settings.aiSettings.providerOptions.openai")}
              </option>
              <option value="deepseek">
                {t("settings.aiSettings.providerOptions.deepseek")}
              </option>
              <option value="ollama">
                {t("settings.aiSettings.providerOptions.ollama")}
              </option>
            </select>
          </label>

          <label className="form-control gap-2">
            <span className="font-semibold">
              {t("settings.aiSettings.modelName")}
            </span>
            <input
              type="text"
              className="input input-bordered w-full"
              value={form.llm.modelName}
              onChange={(event) => {
                const modelName = event.target.value;
                setFeedback(null);
                setForm((current) =>
                  current
                    ? {
                        ...current,
                        llm: {
                          ...current.llm,
                          modelName,
                        },
                      }
                    : current,
                );
              }}
            />
          </label>

          <label className="form-control gap-2 md:col-span-2">
            <span className="font-semibold">
              {t("settings.aiSettings.baseUrl")}
            </span>
            <input
              type="text"
              className="input input-bordered w-full"
              value={form.llm.baseURL}
              onChange={(event) => {
                const baseURL = event.target.value;
                setFeedback(null);
                setForm((current) =>
                  current
                    ? {
                        ...current,
                        llm: {
                          ...current.llm,
                          baseURL,
                        },
                      }
                    : current,
                );
              }}
            />
            <span className="text-sm text-base-content/70">
              {t("settings.aiSettings.baseUrlDescription")}
            </span>
          </label>

          <label className="form-control gap-2 md:col-span-2">
            <span className="font-semibold">
              {t("settings.aiSettings.apiKey")}
            </span>
            <input
              type="password"
              className="input input-bordered w-full"
              value={form.llm.apiKey}
              disabled={form.llm.clearStoredKey}
              placeholder={t("settings.aiSettings.apiKeyPlaceholder")}
              onChange={(event) => {
                const apiKey = event.target.value;
                setFeedback(null);
                setForm((current) =>
                  current
                    ? {
                        ...current,
                        llm: {
                          ...current.llm,
                          apiKey,
                        },
                      }
                    : current,
                );
              }}
            />
            <span className="text-sm text-base-content/70">
              {t("settings.aiSettings.apiKeyDescription")}
            </span>
          </label>
        </div>

        <label className="flex items-center gap-3">
          <input
            type="checkbox"
            className="checkbox checkbox-primary"
            checked={form.llm.clearStoredKey}
            onChange={(event) => {
              const clearStoredKey = event.target.checked;
              setFeedback(null);
              setForm((current) =>
                current
                  ? {
                      ...current,
                      llm: {
                        ...current.llm,
                        clearStoredKey,
                        apiKey: clearStoredKey ? "" : current.llm.apiKey,
                      },
                    }
                  : current,
              );
            }}
          />
          <span>{t("settings.aiSettings.clearStoredKey")}</span>
        </label>

        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            className="btn btn-primary"
            disabled={!isDirty || isBusy}
            onClick={() => void persistSettings()}
          >
            {updateMutation.isPending ? t("common.loading") : t("common.save")}
          </button>
          <button
            type="button"
            className="btn btn-outline"
            disabled={isBusy}
            onClick={() => void handleValidate()}
          >
            {validateMutation.isPending
              ? t("common.loading")
              : t("settings.aiSettings.validate")}
          </button>
        </div>
      </section>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-5">
        <div className="space-y-1">
          <h3 className="text-lg font-semibold">
            {t("settings.aiSettings.mlTitle")}
          </h3>
          <p className="text-sm text-base-content/70">
            {t("settings.aiSettings.mlDescription")}
          </p>
        </div>

        <div className="rounded-2xl border border-base-300 bg-base-100 p-4 space-y-3">
          <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
            <div className="space-y-1">
              <h4 className="font-semibold">
                {t("settings.aiSettings.runtimeTitle")}
              </h4>
              <p className="text-sm text-base-content/70">
                {t("settings.aiSettings.runtimeDescription")}
              </p>
            </div>

            {capabilities && (
              <span className="badge badge-outline">
                {t("settings.aiSettings.nodesSummary", {
                  active: capabilities.ml.activeNodeCount,
                  discovered: capabilities.ml.discoveredNodeCount,
                })}
              </span>
            )}
          </div>

          {capabilities ? (
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={runtimeBadgeClass({
                  enabled: true,
                  available: semanticSearchRuntimeReady,
                })}
              >
                {t("settings.aiSettings.searchRuntime")}:{" "}
                {semanticSearchRuntimeReady
                  ? t("settings.serverSettings.available")
                  : t("settings.serverSettings.unavailable")}
              </span>
              <span className="badge badge-outline">
                {t(
                  `settings.serverSettings.autoModeValues.${capabilities.ml.autoMode}`,
                )}
              </span>
            </div>
          ) : (
            <div className="text-sm text-base-content/70">
              {capabilitiesQuery.isError
                ? t("settings.aiSettings.runtimeUnavailable")
                : t("common.loading")}
            </div>
          )}
        </div>

        <div className="rounded-2xl border border-base-300 bg-base-100 p-4 space-y-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="space-y-1">
              <h4 className="font-semibold">
                {t("settings.aiSettings.coverageTitle")}
              </h4>
              <p className="text-sm text-base-content/70">
                {t("settings.aiSettings.coverageDescription")}
              </p>
            </div>

            {indexingStats && (
              <div className="flex flex-wrap items-center gap-2">
                <span className="badge badge-outline">
                  {t("settings.aiSettings.photoTotalStatus")}:{" "}
                  {indexingStats.photoTotal}
                </span>
                <span className="badge badge-outline">
                  {t("settings.aiSettings.reindexJobsStatus")}:{" "}
                  {indexingStats.reindexJobs}
                </span>
              </div>
            )}
          </div>

          {indexingStatsQuery.isError ? (
            <div className="text-sm text-base-content/70">
              {t("settings.aiSettings.indexingStatsUnavailable")}
            </div>
          ) : (
            <div className="space-y-4">
              <p className="text-sm text-base-content/70">
                {t("settings.aiSettings.queueMissingAssetsHint", {
                  limit: 200,
                })}
              </p>

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="space-y-2">
                  <span className="font-semibold">
                    {t("settings.aiSettings.repositoryScopeLabel")}
                  </span>
                  <div className="rounded-xl border border-base-300 bg-base-100 px-4 py-3">
                    <div className="font-medium">{scopeLabel}</div>
                    <div className="mt-1 text-sm text-base-content/70">
                      {scopeDescription}
                    </div>
                  </div>
                </div>

                <div className="space-y-2">
                  <div className="font-semibold">
                    {t("settings.aiSettings.reindexTasksLabel")}
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {mlTasks.map(({ statsKey, key, label }) => {
                      const selectable =
                        form.ml.autoMode === "enable" || form.ml[key];
                      const checked = selectedReindexTasks.includes(statsKey);

                      return (
                        <label
                          key={`reindex-${statsKey}`}
                          className={[
                            "flex items-center gap-2 rounded-xl border px-3 py-2 text-sm",
                            selectable
                              ? "border-base-300 bg-base-100"
                              : "border-base-200 bg-base-200 text-base-content/50",
                          ].join(" ")}
                        >
                          <input
                            type="checkbox"
                            className="checkbox checkbox-primary checkbox-sm"
                            checked={checked}
                            disabled={!selectable || rebuildMutation.isPending}
                            onChange={(event) =>
                              toggleReindexTask(statsKey, event.target.checked)
                            }
                          />
                          <span>{label}</span>
                        </label>
                      );
                    })}
                  </div>
                  <p className="text-sm text-base-content/70">
                    {form.ml.autoMode === "enable"
                      ? t("settings.aiSettings.reindexTasksAutoHint")
                      : t("settings.aiSettings.reindexTasksManualHint")}
                  </p>
                </div>
              </div>
            </div>
          )}

          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              className="btn btn-outline"
              disabled={
                rebuildMutation.isPending ||
                indexingStatsQuery.isLoading ||
                isDirty ||
                selectedReindexTasks.length === 0
              }
              onClick={() => void handleQueueReindex()}
            >
              {rebuildMutation.isPending
                ? t("settings.aiSettings.queueMissingAssetsPending")
                : t("settings.aiSettings.queueMissingAssets")}
            </button>

            <span className="text-sm text-base-content/70">
              {isDirty
                ? t("settings.aiSettings.saveBeforeReindexHint")
                : t("settings.aiSettings.existingAssetsHint")}
            </span>
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          <button
            type="button"
            className={[
              "rounded-2xl border p-4 text-left transition",
              form.ml.autoMode === "enable"
                ? "border-primary bg-primary/8 shadow-sm"
                : "border-base-300 bg-base-100 hover:border-base-content/30",
            ].join(" ")}
            onClick={() => {
              setFeedback(null);
              setForm((current) =>
                current
                  ? {
                      ...current,
                      ml: {
                        ...current.ml,
                        autoMode: "enable",
                      },
                    }
                  : current,
              );
            }}
          >
            <div className="font-semibold">
              {t("settings.aiSettings.autoMode")}
            </div>
            <div className="mt-2 text-sm text-base-content/70">
              {t("settings.aiSettings.autoModeDescription")}
            </div>
          </button>

          <button
            type="button"
            className={[
              "rounded-2xl border p-4 text-left transition",
              form.ml.autoMode === "disable"
                ? "border-primary bg-primary/8 shadow-sm"
                : "border-base-300 bg-base-100 hover:border-base-content/30",
            ].join(" ")}
            onClick={() => {
              setFeedback(null);
              setForm((current) =>
                current
                  ? {
                      ...current,
                      ml: {
                        ...current.ml,
                        autoMode: "disable",
                      },
                    }
                  : current,
              );
            }}
          >
            <div className="font-semibold">
              {t("settings.aiSettings.manualMode")}
            </div>
            <div className="mt-2 text-sm text-base-content/70">
              {t("settings.aiSettings.manualModeDescription")}
            </div>
          </button>
        </div>

        <div className="space-y-3">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h4 className="font-semibold">
                {t("settings.aiSettings.tasksTitle")}
              </h4>
              <p className="text-sm text-base-content/70">
                {t("settings.aiSettings.tasksDescription")}
              </p>
            </div>
            {form.ml.autoMode === "enable" && (
              <span className="badge badge-outline">
                {t("settings.aiSettings.autoOverrideHint")}
              </span>
            )}
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            {mlTasks.map(
              ({
                statsKey,
                key,
                label,
                description,
                runtime,
                queryRuntime,
              }) => {
                const taskStats = indexingStats?.tasks[statsKey];

                return (
                  <div
                    key={key}
                    className="rounded-2xl border border-base-300 bg-base-100 p-4 flex items-start justify-between gap-4"
                  >
                    <div className="space-y-3">
                      <div>
                        <div className="font-semibold">{label}</div>
                        <p className="mt-1 text-sm text-base-content/70">
                          {description}
                        </p>
                      </div>

                      <div className="flex flex-wrap gap-2">
                        <span className={configuredBadgeClass(form.ml[key])}>
                          {t("settings.aiSettings.configuredStatus")}:{" "}
                          {form.ml[key]
                            ? t("settings.serverSettings.enabled")
                            : t("settings.serverSettings.disabled")}
                        </span>
                        <span className={runtimeBadgeClass(runtime)}>
                          {t("settings.aiSettings.runtimeStatus")}:{" "}
                          {runtimeAvailabilityLabel(runtime)}
                        </span>
                        {queryRuntime && (
                          <span className={runtimeBadgeClass(queryRuntime)}>
                            {t("settings.aiSettings.queryRuntimeStatus")}:{" "}
                            {runtimeAvailabilityLabel(queryRuntime)}
                          </span>
                        )}
                        {taskStats && (
                          <span
                            className={coverageBadgeClass(taskStats.coverage)}
                          >
                            {t("settings.aiSettings.coverageStatus")}:{" "}
                            {t("settings.aiSettings.coverageValue", {
                              indexed: taskStats.indexedCount,
                              total: indexingStats.photoTotal,
                              percent: formatCoveragePercent(
                                taskStats.coverage,
                              ),
                            })}
                          </span>
                        )}
                        {taskStats && (
                          <span className="badge badge-outline">
                            {t("settings.aiSettings.queuedJobsStatus")}:{" "}
                            {taskStats.queuedJobs}
                          </span>
                        )}
                      </div>
                    </div>

                    <input
                      type="checkbox"
                      className="toggle toggle-primary"
                      checked={form.ml[key]}
                      disabled={form.ml.autoMode === "enable"}
                      onChange={(event) => {
                        const checked = event.target.checked;
                        setFeedback(null);
                        setForm((current) =>
                          current
                            ? {
                                ...current,
                                ml: {
                                  ...current.ml,
                                  [key]: checked,
                                },
                              }
                            : current,
                        );
                      }}
                    />
                  </div>
                );
              },
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
