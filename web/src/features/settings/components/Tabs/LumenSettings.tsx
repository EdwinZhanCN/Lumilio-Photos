import { useSettings } from "@/contexts/SettingsContext";
import React, { useState } from "react";

export default function LumenSettings() {
  const { settings, setSettings, isMobile } = useSettings();

  // Local state for all settings
  const [localSettings, setLocalSettings] = useState({
    enabled: settings.lumen?.enabled ?? true,
    model: settings.lumen?.model || "Qwen3-1.7B-q4f16_1-MLC",
    systemPrompt: settings.lumen?.systemPrompt || "",
    temperature: settings.lumen?.temperature ?? 0.7,
    top_p: settings.lumen?.top_p ?? 0.9,
  });

  const [customModelList, setCustomModelList] = useState(
    JSON.stringify(settings.lumen?.modelRecords || [], null, 2),
  );

  // Update local state and textarea when settings change
  React.useEffect(() => {
    setLocalSettings({
      enabled: settings.lumen?.enabled ?? true,
      model: settings.lumen?.model || "Qwen3-1.7B-q4f16_1-MLC",
      systemPrompt: settings.lumen?.systemPrompt || "",
      temperature: settings.lumen?.temperature ?? 0.7,
      top_p: settings.lumen?.top_p ?? 0.9,
    });
    setCustomModelList(
      JSON.stringify(settings.lumen?.modelRecords || [], null, 2),
    );
  }, [settings.lumen]);

  const handleEnabledChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (isMobile) return; // Prevent enabling on mobile
    setLocalSettings({
      ...localSettings,
      enabled: event.target.checked,
    });
  };

  const handleModelChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setLocalSettings({
      ...localSettings,
      model: event.target.value,
    });
  };

  const handleSystemPromptChange = (
    event: React.ChangeEvent<HTMLTextAreaElement>,
  ) => {
    setLocalSettings({
      ...localSettings,
      systemPrompt: event.target.value,
    });
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
      const modelRecords = JSON.parse(customModelList);
      if (settings.lumen) {
        setSettings({
          ...settings,
          lumen: {
            ...settings.lumen,
            ...localSettings,
            modelRecords,
          },
        });
      }
    } catch (error) {
      console.error("Invalid JSON format for model list:", error);
      alert("Invalid JSON format for model list. Please check your syntax.");
    }
  };

  const hasUnsavedChanges = () => {
    if (!settings.lumen) return false;
    return (
      localSettings.enabled !== (settings.lumen.enabled ?? true) ||
      localSettings.model !==
        (settings.lumen.model || "Qwen3-1.7B-q4f16_1-MLC") ||
      localSettings.systemPrompt !== (settings.lumen.systemPrompt || "") ||
      localSettings.temperature !== (settings.lumen.temperature ?? 0.7) ||
      localSettings.top_p !== (settings.lumen.top_p ?? 0.9)
    );
  };

  return (
    <div className="h-full">
      <h1 className="text-3xl font-bold my-3">Lumen on Browser</h1>
      <p>
        Lumen is an on browser AI assistant, you can chat with it click the
        Lumen avatar on the nav bar. <br /> Also, It gathers the information for
        your bio atalas. This function is auto-disabled on mobile phones.
      </p>
      {isMobile && (
        <div className="alert alert-warning mb-4">
          <span>
            Lumen is disabled on mobile devices due to performance limitations.
          </span>
        </div>
      )}
      <div>
        <h1 className="text-2xl font-bold my-2">Enable</h1>
        {/*Enable Lumen? */}
        <input
          type="checkbox"
          checked={localSettings.enabled}
          onChange={handleEnabledChange}
          disabled={isMobile}
          className={`toggle toggle-primary ${isMobile ? "opacity-50" : ""}`}
        />
      </div>
      {/*Choose Which Model is Enabled? */}
      <div className="flex items-center gap-2">
        <h1 className="text-2xl font-bold my-2">Choose your Model</h1>
      </div>
      <p>
        We highly recommend you using Qwen3 series, they are open-sourced, small
        and powerful. <br />
        However, you can configure your own model list in Advanced settings, we
        use WebLLM as our backbone.
      </p>
      <div className="flex flex-wrap md:flex-nowrap gap-4 py-3 overflow-x-auto">
        {settings.lumen?.modelRecords?.map((modelRecord) => (
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
                VRAM Required: {Math.round(modelRecord.vram_required_MB || 0)}MB
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
      {/*System Prompt */}
      <div>
        <h1 className="text-2xl font-bold my-2">System Prompt (Chat Only)</h1>
        <textarea
          placeholder="Enter your system prompt here..."
          className="textarea textarea-primary w-full h-32"
          value={localSettings.systemPrompt}
          onChange={handleSystemPromptChange}
          disabled={isMobile}
        ></textarea>
      </div>
      <div>
        <h1 className="text-2xl font-bold my-2">Advanced settings</h1>
        <div className="flex flex-col gap-2">
          <label className="flex items-center gap-2">
            <span className="w-32">Temperature</span>
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
            <span className="w-32">Top P</span>
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
        <div>
          <h1 className="text-2xl font-bold my-2">Model List (JSON)</h1>
          <p>
            Check config from{" "}
            <a
              href="https://github.com/mlc-ai/web-llm/blob/main/src/config.ts"
              rel="noreferrer"
              target="_blank"
              className="text-blue-600"
            >
              WebLLM
            </a>
            (Do not edit this unless you know what is it)
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
            Save {hasUnsavedChanges() && "(*)"}
          </button>
          {hasUnsavedChanges() && (
            <div className="text-sm text-warning self-center">
              You have unsaved changes
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
