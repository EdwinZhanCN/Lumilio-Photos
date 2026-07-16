import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { useAISettingsDraft, type AISettingsDraft } from "../../hooks/useAISettingsDraft";
import { useValidateLLMSettings } from "../../hooks/useSystemSettings";
import {
  BirdIcon,
  BotIcon,
  BotMessageSquareIcon,
  EyeIcon,
  KeyRoundIcon,
  LinkIcon,
  ScanFaceIcon,
  SparklesIcon,
  TextSearchIcon,
} from "lucide-react";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../SettingsGroup";
import { SettingsDropdown } from "../SettingsDropdown";
import { SettingsSaveBar } from "../SettingsSaveBar";

type AgentProvider = AISettingsDraft["llm"]["provider"];

type FeedbackState = { tone: "success" | "error"; message: string } | null;

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const maybeApiError = error as { message?: string; error?: string };
    if (maybeApiError.message) return maybeApiError.message;
    if (maybeApiError.error) return maybeApiError.error;
  }
  return fallback;
}

const ML_META = {
  semanticEnabled: {
    icon: <TextSearchIcon className="size-4" />,
    color: "bg-info text-info-content",
  },
  bioclipEnabled: {
    icon: <BirdIcon className="size-4" />,
    color: "bg-success text-success-content",
  },
  ocrEnabled: { icon: <EyeIcon className="size-4" />, color: "bg-warning text-warning-content" },
  faceEnabled: {
    icon: <ScanFaceIcon className="size-4" />,
    color: "bg-secondary text-secondary-content",
  },
} as const;

export default function AiTab() {
  const { t } = useI18n();
  const {
    draft,
    setDraft,
    isDirty,
    isSaving,
    save,
    saveAsync,
    reset,
    saveError,
    justSaved,
    apiKeyConfigured,
    query: settingsQuery,
  } = useAISettingsDraft();
  const validateMutation = useValidateLLMSettings();
  const [feedback, setFeedback] = useState<FeedbackState>(null);

  useEffect(() => {
    if (justSaved) {
      setFeedback({ tone: "success", message: t("settings.aiSettings.saveSuccess") });
    }
  }, [justSaved, t]);

  useEffect(() => {
    if (saveError) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(saveError, t("settings.aiSettings.saveError")),
      });
    }
  }, [saveError, t]);

  const isBusy = isSaving || validateMutation.isPending;

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

  const handleValidate = async () => {
    if (!draft) return;
    setFeedback(null);
    if (isDirty) {
      try {
        await saveAsync();
      } catch (error) {
        setFeedback({
          tone: "error",
          message: getErrorMessage(error, t("settings.aiSettings.saveError")),
        });
        return;
      }
    }
    try {
      await validateMutation.mutateAsync({});
      setFeedback({ tone: "success", message: t("settings.aiSettings.validationSuccess") });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(error, t("settings.aiSettings.validationError")),
      });
    }
  };

  if (settingsQuery.isLoading || !draft) {
    return (
      <div className="w-full rounded-2xl bg-base-200/50 px-4 py-6 text-sm text-base-content/60">
        {t("common.loading")}
      </div>
    );
  }

  if (settingsQuery.isError) {
    return (
      <div className="w-full rounded-2xl bg-warning/10 px-4 py-6 text-sm text-warning">
        {t("settings.aiSettings.loadError")}
      </div>
    );
  }

  return (
    <div className="w-full space-y-8 lg:space-y-10">
      {feedback && (
        <div
          className={`rounded-xl px-4 py-3 text-sm ${
            feedback.tone === "success" ? "bg-success/10 text-success" : "bg-error/10 text-error"
          }`}
        >
          {feedback.message}
        </div>
      )}

      <SettingsGroup
        title={t("settings.aiSettings.agentTitle")}
        description={t("settings.aiSettings.agentDescription")}
      >
        <SettingsRow
          icon={<BotMessageSquareIcon className="size-4" />}
          iconColor="bg-primary text-primary-content"
          label={t("settings.aiSettings.agentEnabledLabel")}
          control={
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={draft.llm.agentEnabled}
              aria-label={t("settings.aiSettings.agentTitle")}
              onChange={(event) => {
                setFeedback(null);
                setDraft({ ...draft, llm: { ...draft.llm, agentEnabled: event.target.checked } });
              }}
            />
          }
        />
        <SettingsRow
          htmlFor="ai-provider"
          icon={<SparklesIcon className="size-4" />}
          iconColor="bg-info text-info-content"
          label={t("settings.aiSettings.provider")}
          control={
            <SettingsDropdown<AgentProvider>
              id="ai-provider"
              value={draft.llm.provider}
              options={[
                { value: "", label: t("settings.aiSettings.providerOptions.unset") },
                { value: "ark", label: t("settings.aiSettings.providerOptions.ark") },
                { value: "openai", label: t("settings.aiSettings.providerOptions.openai") },
                { value: "deepseek", label: t("settings.aiSettings.providerOptions.deepseek") },
                { value: "ollama", label: t("settings.aiSettings.providerOptions.ollama") },
              ]}
              onChange={(provider) => {
                setFeedback(null);
                setDraft({ ...draft, llm: { ...draft.llm, provider } });
              }}
              ariaLabel={t("settings.aiSettings.provider")}
              className="w-40"
            />
          }
        />
        <SettingsBlock>
          <label htmlFor="ai-model" className="flex items-center gap-2 text-sm font-medium">
            <BotIcon className="size-3.5 text-base-content/50" />
            {t("settings.aiSettings.modelName")}
          </label>
          <input
            id="ai-model"
            type="text"
            className="input input-bordered input-sm mt-2 w-full"
            value={draft.llm.modelName}
            onChange={(event) => {
              setFeedback(null);
              setDraft({ ...draft, llm: { ...draft.llm, modelName: event.target.value } });
            }}
          />
        </SettingsBlock>
        <SettingsBlock>
          <label htmlFor="ai-baseurl" className="flex items-center gap-2 text-sm font-medium">
            <LinkIcon className="size-3.5 text-base-content/50" />
            {t("settings.aiSettings.baseUrl")}
          </label>
          <input
            id="ai-baseurl"
            type="text"
            className="input input-bordered input-sm mt-2 w-full"
            autoComplete="off"
            spellCheck={false}
            value={draft.llm.baseURL}
            onChange={(event) => {
              setFeedback(null);
              setDraft({ ...draft, llm: { ...draft.llm, baseURL: event.target.value } });
            }}
          />
          <p className="mt-1.5 text-xs text-base-content/55">
            {t("settings.aiSettings.baseUrlDescription")}
          </p>
        </SettingsBlock>
        <SettingsBlock>
          <label htmlFor="ai-apikey" className="flex items-center gap-2 text-sm font-medium">
            <KeyRoundIcon className="size-3.5 text-base-content/50" />
            {t("settings.aiSettings.apiKey")}
          </label>
          <input
            id="ai-apikey"
            type="text"
            className="input input-bordered input-sm mt-2 w-full"
            autoComplete="off"
            spellCheck={false}
            value={draft.llm.apiKey}
            disabled={draft.llm.clearStoredKey}
            placeholder={t("settings.aiSettings.apiKeyPlaceholder")}
            onChange={(event) => {
              setFeedback(null);
              setDraft({ ...draft, llm: { ...draft.llm, apiKey: event.target.value } });
            }}
          />
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-base-content/55">
            <span>
              {t("settings.aiSettings.apiKeyConfigured")}:{" "}
              <span className="font-medium text-base-content">
                {t(`settings.serverSettings.booleanValues.${apiKeyConfigured ? "true" : "false"}`)}
              </span>
            </span>
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                className="checkbox checkbox-primary checkbox-xs"
                checked={draft.llm.clearStoredKey}
                onChange={(event) => {
                  const clearStoredKey = event.target.checked;
                  setFeedback(null);
                  setDraft({
                    ...draft,
                    llm: {
                      ...draft.llm,
                      clearStoredKey,
                      apiKey: clearStoredKey ? "" : draft.llm.apiKey,
                    },
                  });
                }}
              />
              <span>{t("settings.aiSettings.clearStoredKey")}</span>
            </label>
          </div>
        </SettingsBlock>
      </SettingsGroup>

      <SettingsGroup title={t("settings.aiSettings.mlTitle")}>
        {mlTasks.map(({ key, label, description }) => (
          <SettingsRow
            key={key}
            align="start"
            icon={ML_META[key].icon}
            iconColor={ML_META[key].color}
            label={label}
            description={description}
            control={
              <input
                type="checkbox"
                className="toggle toggle-primary"
                checked={draft.ml[key]}
                aria-label={label}
                onChange={(event) => {
                  const checked = event.target.checked;
                  setFeedback(null);
                  setDraft({ ...draft, ml: { ...draft.ml, [key]: checked } });
                }}
              />
            }
          />
        ))}
      </SettingsGroup>

      <SettingsSaveBar
        isDirty={isDirty}
        isSaving={isSaving}
        justSaved={justSaved}
        error={saveError}
        canSave={isDirty && !isBusy}
        onSave={() => {
          setFeedback(null);
          save();
        }}
        onReset={() => {
          setFeedback(null);
          reset();
        }}
        extraAction={
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={isBusy}
            onClick={() => void handleValidate()}
          >
            {validateMutation.isPending ? t("common.loading") : t("settings.aiSettings.validate")}
          </button>
        }
      />
    </div>
  );
}
