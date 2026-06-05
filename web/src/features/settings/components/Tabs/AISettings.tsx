import { useEffect, useMemo, useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useSystemSettings,
  useUpdateSystemSettings,
  useValidateLLMSettings,
  type SystemSettings,
  type UpdateSystemSettingsPayload,
} from "@/features/settings/hooks/useSystemSettings";
import { SaveIcon, SparklesIcon } from "lucide-react";

type AgentProvider = SystemSettings["llm"]["provider"];

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
    semanticEnabled: boolean;
    bioclipEnabled: boolean;
    ocrEnabled: boolean;
    faceEnabled: boolean;
  };
};

type FeedbackState = {
  tone: "success" | "error";
  message: string;
} | null;

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
      semanticEnabled: settings.ml.semanticEnabled,
      bioclipEnabled: settings.ml.bioclipEnabled,
      ocrEnabled: settings.ml.ocrEnabled,
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
      semantic_enabled: form.ml.semanticEnabled,
      bioclip_enabled: form.ml.bioclipEnabled,
      ocr_enabled: form.ml.ocrEnabled,
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
  const settingsQuery = useSystemSettings();
  const updateMutation = useUpdateSystemSettings();
  const validateMutation = useValidateLLMSettings();
  const settings = settingsQuery.settings;
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
    settings?.ml.semanticEnabled,
    settings?.ml.bioclipEnabled,
    settings?.ml.ocrEnabled,
    settings?.ml.faceEnabled,
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
      form.ml.semanticEnabled !== settings.ml.semanticEnabled ||
      form.ml.bioclipEnabled !== settings.ml.bioclipEnabled ||
      form.ml.ocrEnabled !== settings.ml.ocrEnabled ||
      form.ml.faceEnabled !== settings.ml.faceEnabled
    );
  }, [form, settings]);

  const isBusy = updateMutation.isPending || validateMutation.isPending;

  const mlTasks = [
    {
      key: "semanticEnabled",
      label: t("settings.aiSettings.taskNames.semantic"),
      description: t("settings.aiSettings.taskDescriptions.semantic"),
    },
    {
      key: "bioclipEnabled",
      label: t("settings.aiSettings.taskNames.bioclip"),
      description: t("settings.aiSettings.taskDescriptions.bioclip"),
    },
    {
      key: "ocrEnabled",
      label: t("settings.aiSettings.taskNames.ocr"),
      description: t("settings.aiSettings.taskDescriptions.ocr"),
    },
    {
      key: "faceEnabled",
      label: t("settings.aiSettings.taskNames.face"),
      description: t("settings.aiSettings.taskDescriptions.face"),
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
      <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <SparklesIcon className="size-6 text-primary" />
            <h2 className="text-2xl font-bold">{t("settings.ai")}</h2>
          </div>
          <p className="text-base-content/70">
            {t("settings.aiSettings.description")}
          </p>
        </div>
        <button
          type="button"
          className="btn btn-primary gap-2 sm:shrink-0"
          disabled={!isDirty || isBusy}
          onClick={() => void persistSettings()}
        >
          {updateMutation.isPending ? (
            <span className="loading loading-spinner loading-xs" />
          ) : (
            <SaveIcon size={16} />
          )}
          {updateMutation.isPending ? t("common.loading") : t("common.save")}
        </button>
      </header>

      {feedback && (
        <div
          className={`alert ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
        >
          <span>{feedback.message}</span>
        </div>
      )}

      <section className="space-y-5 rounded-2xl border border-base-300 bg-base-100 p-5">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold">
              {t("settings.aiSettings.agentTitle")}
            </h3>
            <p className="text-sm text-base-content/70">
              {t("settings.aiSettings.agentDescription")}
            </p>
          </div>
          <input
            type="checkbox"
            className="toggle toggle-primary mt-1 shrink-0"
            checked={form.llm.agentEnabled}
            aria-label={t("settings.aiSettings.agentTitle")}
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
              autoComplete="off"
              spellCheck={false}
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
              type="text"
              className="input input-bordered w-full"
              autoComplete="off"
              spellCheck={false}
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

        <div className="flex flex-col gap-3 rounded-lg bg-base-200/60 px-3 py-3 text-sm sm:flex-row sm:items-center sm:justify-between">
          <span className="text-base-content/70">
            {t("settings.aiSettings.apiKeyConfigured")}:{" "}
            <span className="font-medium text-base-content">
              {t(
                `settings.serverSettings.booleanValues.${settings.llm.apiKeyConfigured ? "true" : "false"}`,
              )}
            </span>
          </span>
          <label className="flex items-center gap-3">
            <input
              type="checkbox"
              className="checkbox checkbox-primary checkbox-sm"
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
        </div>

        <div className="flex flex-wrap items-center gap-3">
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
        <h3 className="text-lg font-semibold">
          {t("settings.aiSettings.mlTitle")}
        </h3>

        <div className="divide-y divide-base-300 rounded-2xl border border-base-300 bg-base-100">
          {mlTasks.map(({ key, label, description }) => (
            <div
              key={key}
              className="flex items-center justify-between gap-4 px-4 py-4"
            >
              <div className="min-w-0">
                <div className="font-semibold">{label}</div>
                <p className="mt-1 text-sm text-base-content/70">
                  {description}
                </p>
              </div>

              <input
                type="checkbox"
                className="toggle toggle-primary shrink-0"
                checked={form.ml[key]}
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
          ))}
        </div>
      </section>
    </div>
  );
}
