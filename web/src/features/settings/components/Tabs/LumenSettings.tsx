import { useSettingsContext } from "@/features/settings";
import { ModelRecord } from "@mlc-ai/web-llm";
import React, { useEffect, useState } from "react";
import {
  SparklesIcon,
  CpuChipIcon,
  Cog6ToothIcon,
} from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";

export default function LumenSettings() {
  const { t } = useI18n();
  const { state, dispatch } = useSettingsContext();
  const { lumen: lumenSettings } = state;

  // A simple isMobile detection. In a real-world app, this might be a more robust custom hook.
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const checkIsMobile = () => window.innerWidth < 768;
    setIsMobile(checkIsMobile());
    window.addEventListener("resize", checkIsMobile);
    return () => window.removeEventListener("resize", checkIsMobile);
  }, []);

  // Local state for form elements to allow for "Save" and "Cancel" functionality
  const [localSettings, setLocalSettings] = useState(lumenSettings);
  const [customModelList, setCustomModelList] = useState(
    JSON.stringify(lumenSettings.modelRecords || [], null, 2),
  );

  // When the settings from the context change, update the local state
  useEffect(() => {
    setLocalSettings(lumenSettings);
    setCustomModelList(
      JSON.stringify(lumenSettings.modelRecords || [], null, 2),
    );
  }, [lumenSettings]);

  const handleEnabledChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (isMobile) return;
    setLocalSettings({ ...localSettings, enabled: event.target.checked });
  };

  const handleModelChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setLocalSettings({ ...localSettings, model: event.target.value });
  };

  const handleSystemPromptChange = (
    event: React.ChangeEvent<HTMLTextAreaElement>,
  ) => {
    setLocalSettings({ ...localSettings, systemPrompt: event.target.value });
  };

  const handleTemperatureChange = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    setLocalSettings({
      ...localSettings,
      temperature: parseFloat(event.target.value),
    });
  };

  const handleTopPChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setLocalSettings({
      ...localSettings,
      top_p: parseFloat(event.target.value),
    });
  };

  const handleSave = () => {
    try {
      const modelRecords: ModelRecord[] = JSON.parse(customModelList);
      dispatch({
        type: "SET_LUMEN_ENABLED",
        payload: localSettings.enabled ?? true,
      });
      dispatch({ type: "SET_LUMEN_MODEL", payload: localSettings.model });
      dispatch({
        type: "SET_LUMEN_SYSTEM_PROMPT",
        payload: localSettings.systemPrompt ?? "",
      });
      dispatch({
        type: "SET_LUMEN_TEMPERATURE",
        payload: localSettings.temperature,
      });
      dispatch({ type: "SET_LUMEN_TOP_P", payload: localSettings.top_p });
      dispatch({ type: "SET_LUMEN_MODELRECORDS", payload: modelRecords });
    } catch (error) {
      console.error("Invalid JSON format for model list:", error);
      alert("Invalid JSON format for model list. Please check your syntax.");
    }
  };

  const hasUnsavedChanges = () => {
    let modelsChanged = false;
    try {
      const customModels = JSON.parse(customModelList);
      modelsChanged =
        JSON.stringify(customModels) !==
        JSON.stringify(lumenSettings.modelRecords ?? []);
    } catch (e) {
      console.log("Invalid JSON format", e);
      return true; // Invalid JSON is an unsaved change
    }

    return (
      (localSettings.enabled ?? true) !== (lumenSettings.enabled ?? true) ||
      localSettings.model !== lumenSettings.model ||
      (localSettings.systemPrompt ?? "") !==
        (lumenSettings.systemPrompt ?? "") ||
      localSettings.temperature !== lumenSettings.temperature ||
      localSettings.top_p !== lumenSettings.top_p ||
      modelsChanged
    );
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <SparklesIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">
          {t("settings.lumenSettings.title")}
        </h2>
      </div>
      <p>{t("settings.lumenSettings.description")}</p>
      {isMobile && (
        <div className="alert alert-warning mb-4">
          <span>{t("settings.lumenSettings.mobileDisabledWarning")}</span>
        </div>
      )}

      <section className="space-y-3">
        <h3 className="text-lg font-semibold">
          {t("settings.lumenSettings.enable")}
        </h3>
        <input
          type="checkbox"
          checked={localSettings.enabled ?? true}
          onChange={handleEnabledChange}
          disabled={isMobile}
          className={`toggle toggle-primary ${isMobile ? "opacity-50" : ""}`}
        />
      </section>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <CpuChipIcon className="size-6 text-primary" />
          <h3 className="text-lg font-semibold">
            {t("settings.lumenSettings.chooseModel")}
          </h3>
        </div>
        <p>{t("settings.lumenSettings.modelDescription")}</p>
        <div className="flex flex-wrap md:flex-nowrap gap-4 py-3 overflow-x-auto">
          {lumenSettings.modelRecords?.map((modelRecord) => (
            <div
              key={modelRecord.model_id}
              className="card bg-base-100 shadow-sm flex-1 max-w-[500px]"
            >
              {(modelRecord.model_id === "Qwen3-4B-q4f16_1-MLC" ||
                modelRecord.model_id === "Qwen3-1.7B-q4f16_1-MLC") && (
                <figure className="px-10 pt-10">
                  <img
                    src="https://qianwen-res.oss-accelerate-overseas.aliyuncs.com/logo_qwen3.png"
                    alt={modelRecord.model_id}
                    className="rounded-xl"
                  />
                </figure>
              )}
              <div className="card-body items-center text-center">
                <h2 className="card-title">{modelRecord.model_id}</h2>
                <p>
                  {t("settings.lumenSettings.vramRequired")}:{" "}
                  {Math.round(modelRecord.vram_required_MB || 0)}
                  MB
                </p>
                <div className="card-actions">
                  <input
                    type="radio"
                    name="radio-4"
                    className="radio radio-primary"
                    value={modelRecord.model_id}
                    checked={localSettings.model === modelRecord.model_id}
                    onChange={handleModelChange}
                    disabled={isMobile}
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-4">
        <h3 className="text-lg font-semibold">
          {t("settings.lumenSettings.systemPrompt")}
        </h3>
        <textarea
          placeholder={t("settings.lumenSettings.systemPromptPlaceholder")}
          className="textarea textarea-primary w-full h-32"
          value={localSettings.systemPrompt ?? ""}
          onChange={handleSystemPromptChange}
          disabled={isMobile}
        ></textarea>
      </section>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Cog6ToothIcon className="size-6 text-primary" />
          <h3 className="text-lg font-semibold">
            {t("settings.lumenSettings.advancedSettings")}
          </h3>
        </div>
        <div className="flex flex-col gap-2">
          <label className="flex items-center gap-2">
            <span className="w-32">
              {t("settings.lumenSettings.temperature")}
            </span>
            <input
              type="range"
              min={0}
              max={1}
              step={0.1}
              value={localSettings.temperature}
              onChange={handleTemperatureChange}
              className="range range-primary"
              disabled={isMobile}
            />
            <span>{localSettings.temperature}</span>
          </label>
          <label className="flex items-center gap-2">
            <span className="w-32">{t("settings.lumenSettings.topP")}</span>
            <input
              type="range"
              min={0}
              max={1}
              step={0.1}
              value={localSettings.top_p}
              onChange={handleTopPChange}
              className="range range-primary"
              disabled={isMobile}
            />
            <span>{localSettings.top_p}</span>
          </label>
        </div>
        <div className="space-y-3">
          <h4 className="text-base font-medium">
            {t("settings.lumenSettings.modelList")}
          </h4>
          <p>
            {t("settings.lumenSettings.modelListDescription")}{" "}
            <a
              href="https://github.com/mlc-ai/web-llm/blob/main/src/config.ts"
              rel="noreferrer"
              target="_blank"
              className="text-blue-600"
            >
              WebLLM
            </a>
          </p>
          <textarea
            className="textarea textarea-primary w-full h-64"
            value={customModelList}
            onChange={(e) => setCustomModelList(e.target.value)}
            disabled={isMobile}
          ></textarea>
        </div>
        <div className="flex gap-2 mt-4">
          <button
            className={`btn btn-primary ${hasUnsavedChanges() ? "btn-primary" : "btn-disabled"}`}
            onClick={handleSave}
            disabled={!hasUnsavedChanges() || isMobile}
          >
            {t("settings.lumenSettings.save")} {hasUnsavedChanges() && "(*)"}
          </button>
          {hasUnsavedChanges() && (
            <div className="text-sm text-warning self-center">
              {t("settings.lumenSettings.unsavedChanges")}
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
