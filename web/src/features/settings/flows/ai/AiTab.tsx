import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { localizeProblem } from "@/lib/http-commons/problem";
import { useAISettingsDraft, type AISettingsDraft } from "./useAISettingsDraft";
import {
  BirdIcon,
  BotIcon,
  BotMessageSquareIcon,
  EyeIcon,
  EyeOffIcon,
  FilmIcon,
  KeyRoundIcon,
  LinkIcon,
  ScanFaceIcon,
  SparklesIcon,
  TextSearchIcon,
} from "lucide-react";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../../components/SettingsGroup";
import { SettingsDropdown } from "../../components/SettingsDropdown";
import { SettingsSaveBar } from "../../components/SettingsSaveBar";
import type { LLMProvider } from "../../model/llmProviders";

type AgentProvider = AISettingsDraft["llm"]["provider"];

type FeedbackState = { tone: "success" | "error"; message: string } | null;

const ML_META = {
  semanticEnabled: {
    icon: <TextSearchIcon className="size-4" />,
    color: "bg-info text-info-content",
  },
  videoSemanticEnabled: {
    icon: <FilmIcon className="size-4" />,
    color: "bg-accent text-accent-content",
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
    reset,
    saveError,
    justSaved,
    apiKeyConfigured,
    supportedProviders,
    query: settingsQuery,
    isValidating,
    validateDraft,
  } = useAISettingsDraft();
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [showAPIKey, setShowAPIKey] = useState(false);
  const providerLabels: Record<LLMProvider, string> = {
    ark: t("settings.aiSettings.providerOptions.ark", "Ark"),
    openai: t("settings.aiSettings.providerOptions.openai", "OpenAI"),
    deepseek: t("settings.aiSettings.providerOptions.deepseek", "DeepSeek"),
    ollama: t("settings.aiSettings.providerOptions.ollama", "Ollama"),
    claude: t("settings.aiSettings.providerOptions.claude", "Claude"),
    gemini: t("settings.aiSettings.providerOptions.gemini", "Gemini"),
    qwen: t("settings.aiSettings.providerOptions.qwen", "Qwen"),
    openrouter: t("settings.aiSettings.providerOptions.openrouter", "OpenRouter"),
  };

  useEffect(() => {
    if (justSaved) {
      setFeedback({ tone: "success", message: t("settings.aiSettings.saveSuccess") });
    }
  }, [justSaved, t]);

  useEffect(() => {
    if (saveError) {
      setFeedback({
        tone: "error",
        message: localizeProblem(saveError, t, t("settings.aiSettings.saveError")),
      });
    }
  }, [saveError, t]);

  const isBusy = isSaving || isValidating;

  const mlTasks = [
    {
      key: "semanticEnabled",
      label: t("settings.aiSettings.taskNames.semantic", "Image Semantic Analysis"),
      description: t("settings.aiSettings.taskDescriptions.semantic"),
    },
    {
      key: "videoSemanticEnabled",
      label: t("settings.aiSettings.taskNames.videoSemantic", "Image Semantic Analysis (video)"),
      description: t(
        "settings.aiSettings.taskDescriptions.videoSemantic",
        "Embed sampled frames from videos for text search. Requires Image Semantic Analysis.",
      ),
    },
    {
      key: "bioclipEnabled",
      label: t("settings.aiSettings.taskNames.bioclip", "BioCLIP Species Recognition"),
      description: t("settings.aiSettings.taskDescriptions.bioclip"),
    },
    {
      key: "ocrEnabled",
      label: t("settings.aiSettings.taskNames.ocr", "OCR Text Recognition"),
      description: t("settings.aiSettings.taskDescriptions.ocr"),
    },
    {
      key: "faceEnabled",
      label: t("settings.aiSettings.taskNames.face", "Person Recognition"),
      description: t("settings.aiSettings.taskDescriptions.face"),
    },
  ] as const;

  const handleValidate = async () => {
    if (!draft) return;
    setFeedback(null);
    try {
      await validateDraft();
      setFeedback({ tone: "success", message: t("settings.aiSettings.validationSuccess") });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: localizeProblem(error, t, t("settings.aiSettings.validationError")),
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
                { value: "none", label: t("settings.aiSettings.providerOptions.unset") },
                ...supportedProviders.map(({ id }) => ({
                  value: id,
                  label: providerLabels[id],
                })),
              ]}
              onChange={(provider) => {
                setFeedback(null);
                setDraft({
                  ...draft,
                  llm: {
                    ...draft.llm,
                    provider,
                    agentEnabled: provider === "none" ? false : draft.llm.agentEnabled,
                    // A stored secret belongs to the previous provider. The
                    // server clears it on provider change unless a replacement
                    // is supplied in the same save.
                    apiKey: "",
                    clearStoredKey: false,
                  },
                });
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
          <div className="relative mt-2">
            <input
              id="ai-apikey"
              type={showAPIKey ? "text" : "password"}
              className="input input-bordered input-sm w-full pr-10"
              autoComplete="new-password"
              spellCheck={false}
              value={draft.llm.apiKey}
              disabled={draft.llm.clearStoredKey}
              placeholder={t("settings.aiSettings.apiKeyPlaceholder")}
              onChange={(event) => {
                setFeedback(null);
                setDraft({ ...draft, llm: { ...draft.llm, apiKey: event.target.value } });
              }}
            />
            <button
              type="button"
              className="btn btn-ghost btn-xs absolute right-1 top-1/2 -translate-y-1/2"
              disabled={draft.llm.clearStoredKey}
              aria-label={
                showAPIKey
                  ? t("settings.aiSettings.hideApiKey", "Hide API key")
                  : t("settings.aiSettings.showApiKey", "Show API key")
              }
              onClick={() => setShowAPIKey((value) => !value)}
            >
              {showAPIKey ? <EyeOffIcon className="size-3.5" /> : <EyeIcon className="size-3.5" />}
            </button>
          </div>
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
        {mlTasks
          // Video semantic search is a sub-capability of semantic search (it
          // reuses the same model, space, and query path), so it is only shown
          // once semantic search itself is enabled.
          .filter(({ key }) => key !== "videoSemanticEnabled" || draft.ml.semanticEnabled)
          .map(({ key, label, description }) => (
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
                    const nextMl = { ...draft.ml, [key]: checked };
                    // Turning the parent off also turns off the subordinate
                    // video toggle so a hidden "on" value is never persisted.
                    if (key === "semanticEnabled" && !checked) {
                      nextMl.videoSemanticEnabled = false;
                    }
                    setDraft({ ...draft, ml: nextMl });
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
            {isValidating ? t("common.loading") : t("settings.aiSettings.validate")}
          </button>
        }
      />
    </div>
  );
}
