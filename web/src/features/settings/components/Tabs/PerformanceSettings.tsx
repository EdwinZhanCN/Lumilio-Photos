import { useState, useEffect } from "react";
import {
  usePerformancePreferences,
  PerformanceProfile,
  PerformancePreferences,
} from "@/utils/performancePreferences";
import { detectDeviceCapabilities } from "@/utils/smartBatchSizing";

export default function PerformanceSettings() {
  const { preferences, updatePreferences, resetToDefaults } =
    usePerformancePreferences();
  const [localPreferences, setLocalPreferences] =
    useState<PerformancePreferences>(preferences);
  const deviceInfo = detectDeviceCapabilities();

  useEffect(() => {
    setLocalPreferences(preferences);
  }, [preferences]);

  const handleProfileChange = (profile: PerformanceProfile) => {
    const updated = { ...localPreferences, profile };
    setLocalPreferences(updated);
    updatePreferences(updated);
  };

  const handleToggleChange = (
    key: keyof PerformancePreferences,
    value: boolean,
  ) => {
    const updated = { ...localPreferences, [key]: value };
    setLocalPreferences(updated);
    updatePreferences(updated);
  };

  const handleNumberChange = (
    key: keyof PerformancePreferences,
    value: number,
  ) => {
    const updated = { ...localPreferences, [key]: value };
    setLocalPreferences(updated);
    updatePreferences(updated);
  };

  const handleReset = () => {
    resetToDefaults();
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold mb-2">Performance Settings</h1>
        <p className="text-base-content/70">
          Configure how Lumilio processes images to balance memory usage and
          speed based on your device capabilities and preferences.
        </p>
      </div>

      {/* Device Information */}
      <div className="card bg-base-100 shadow-sm">
        <div className="card-body">
          <h2 className="card-title text-lg">Device Information</h2>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="font-medium">CPU Cores:</span>{" "}
              {deviceInfo.cpuCores}
            </div>
            <div>
              <span className="font-medium">Available Memory:</span> ~
              {deviceInfo.availableMemoryMB}MB
            </div>
            <div>
              <span className="font-medium">Device Type:</span>
              {deviceInfo.isMobile ? " Mobile" : " Desktop"}
              {deviceInfo.isLowEndDevice && " (Low-end)"}
            </div>
            <div>
              <span className="font-medium">Max Concurrency:</span>{" "}
              {deviceInfo.maxConcurrency}
            </div>
          </div>
        </div>
      </div>

      {/* Performance Profile */}
      <div className="card bg-base-100 shadow-sm">
        <div className="card-body">
          <h2 className="card-title text-lg">Performance Profile</h2>
          <p className="text-sm text-base-content/70 mb-4">
            Choose how to balance memory usage and processing speed
          </p>

          <div className="space-y-3">
            <label className="flex items-center gap-3">
              <input
                type="radio"
                name="performance-profile"
                className="radio radio-primary"
                checked={
                  localPreferences.profile === PerformanceProfile.MEMORY_SAVER
                }
                onChange={() =>
                  handleProfileChange(PerformanceProfile.MEMORY_SAVER)
                }
              />
              <div>
                <div className="font-medium">Memory Saver</div>
                <div className="text-sm text-base-content/70">
                  Minimize memory usage, slower processing
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3">
              <input
                type="radio"
                name="performance-profile"
                className="radio radio-primary"
                checked={
                  localPreferences.profile === PerformanceProfile.BALANCED
                }
                onChange={() =>
                  handleProfileChange(PerformanceProfile.BALANCED)
                }
              />
              <div>
                <div className="font-medium">Balanced</div>
                <div className="text-sm text-base-content/70">
                  Balance between memory usage and speed
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3">
              <input
                type="radio"
                name="performance-profile"
                className="radio radio-primary"
                checked={
                  localPreferences.profile ===
                  PerformanceProfile.SPEED_OPTIMIZED
                }
                onChange={() =>
                  handleProfileChange(PerformanceProfile.SPEED_OPTIMIZED)
                }
              />
              <div>
                <div className="font-medium">Speed Optimized</div>
                <div className="text-sm text-base-content/70">
                  Maximize processing speed, higher memory usage
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3">
              <input
                type="radio"
                name="performance-profile"
                className="radio radio-primary"
                checked={
                  localPreferences.profile === PerformanceProfile.ADAPTIVE
                }
                onChange={() =>
                  handleProfileChange(PerformanceProfile.ADAPTIVE)
                }
              />
              <div>
                <div className="font-medium">Adaptive (Recommended)</div>
                <div className="text-sm text-base-content/70">
                  Automatically adjust based on device capabilities
                </div>
              </div>
            </label>
          </div>
        </div>
      </div>

      {/* Advanced Settings */}
      <div className="card bg-base-100 shadow-sm">
        <div className="card-body">
          <h2 className="card-title text-lg">Advanced Settings</h2>

          <div className="space-y-4">
            <div className="form-control">
              <label className="label cursor-pointer">
                <span className="label-text">
                  <div className="font-medium">Respect Memory Limits</div>
                  <div className="text-sm text-base-content/70">
                    Reduce processing when memory usage is high
                  </div>
                </span>
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={localPreferences.respectMemoryLimits}
                  onChange={(e) =>
                    handleToggleChange("respectMemoryLimits", e.target.checked)
                  }
                />
              </label>
            </div>

            <div className="form-control">
              <label className="label cursor-pointer">
                <span className="label-text">
                  <div className="font-medium">Prioritize User Operations</div>
                  <div className="text-sm text-base-content/70">
                    Give priority to user-visible operations
                  </div>
                </span>
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={localPreferences.prioritizeUserOperations}
                  onChange={(e) =>
                    handleToggleChange(
                      "prioritizeUserOperations",
                      e.target.checked,
                    )
                  }
                />
              </label>
            </div>

            <div className="form-control">
              <label className="label">
                <span className="label-text">
                  <div className="font-medium">Max Concurrent Operations</div>
                  <div className="text-sm text-base-content/70">
                    Maximum number of processing operations running at once
                  </div>
                </span>
              </label>
              <div className="flex items-center gap-4">
                <input
                  type="range"
                  min={1}
                  max={8}
                  value={localPreferences.maxConcurrentOperations}
                  onChange={(e) =>
                    handleNumberChange(
                      "maxConcurrentOperations",
                      parseInt(e.target.value),
                    )
                  }
                  className="range range-primary flex-1"
                />
                <span className="w-8 text-center font-mono">
                  {localPreferences.maxConcurrentOperations}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Performance Tips */}
      <div className="alert alert-info">
        <div>
          <h3 className="font-bold">Performance Tips</h3>
          <div className="text-sm mt-1">
            <ul className="list-disc list-inside space-y-1">
              <li>
                <strong>Memory Saver:</strong> Best for older devices or when
                running many apps
              </li>
              <li>
                <strong>Speed Optimized:</strong> Best for high-end devices with
                plenty of RAM
              </li>
              <li>
                <strong>Adaptive:</strong> Automatically adjusts to your device
                capabilities
              </li>
              <li>
                Changes take effect immediately for new processing operations
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* Reset Button */}
      <div className="flex justify-end">
        <button className="btn btn-outline" onClick={handleReset}>
          Reset to Defaults
        </button>
      </div>
    </div>
  );
}
